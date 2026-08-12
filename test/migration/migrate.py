#!/usr/bin/env python3
"""Helpers for the agentlink -> agenco migration test.

Holds the resource mapping under test and the assertions run.sh makes against Terraform's
machine-readable output. The mapping lives here so the configs, the state surgery and the
README table all trace back to one list.
"""

import hashlib
import json
import sys

# (agentlink address, agenco address, import ID template over the stage-1 outputs)
MAPPING = [
    ("agentlink_application.migration", "agenco_application.migration", "{application_id}"),
    ("agentlink_mcp_configuration.migration", "agenco_mcp_configuration.migration", "{application_id}"),
    ("agentlink_source.migration", "agenco_mcp_source.migration", "{application_id}/{source_id}"),
    ("agentlink_tools_import.migration", "agenco_tools_import.migration", "{application_id}/{source_id}"),
    ("agentlink_masking_policy.migration", "agenco_masking_policy.migration", "{masking_id}"),
    ("agentlink_rbac_policy.migration", "agenco_rbac_policy.migration", "{rbac_id}"),
    ("agentlink_conditional_policy.migration", "agenco_conditional_policy.migration", "{conditional_id}"),
]

DESTRUCTIVE_ACTIONS = {"create", "delete"}

# The only attributes the first apply after migration is allowed to rewrite, because the API cannot
# return what would be needed to reconstruct them at import time. Every other difference between the
# state right after import and the state after that apply is a live value being silently changed —
# which is what a provider default applied on update, rather than only on create, looks like.
EXPECTED_REWRITES = {
    "agenco_tools_import.migration": {"schema_file", "schema_type", "schema_hash", "tools_count"},
    "agenco_rbac_policy.migration": {"enabled", "application_ids"},
}


def load_outputs(path):
    """Reads the JSON written by `terraform output -json migration_ids`.

    Asking for one output by name prints its bare value. Asking for all of them wraps each in a
    {"value", "type"} envelope, and state backups store outputs the same way, so accept both.
    """
    data = json.load(open(path))
    if isinstance(data, dict) and {"value", "type"}.issubset(data):
        return data["value"]
    return data


def cmd_old_addresses():
    for old, _, _ in MAPPING:
        print(old)


def cmd_imports(ids_path):
    ids = load_outputs(ids_path)
    for _, new, template in MAPPING:
        try:
            print(f"{new}\t{template.format(**ids)}")
        except KeyError as missing:
            raise SystemExit(f"error: {ids_path} has no output {missing}, needed to import {new}")


def cmd_assert_plan(plan_path):
    plan = json.load(open(plan_path))
    changes = plan.get("resource_changes", [])
    if not changes:
        print("FAIL: the plan contains no resources, so nothing was imported", file=sys.stderr)
        return 1

    destructive = []
    leftover = []
    for change in changes:
        actions = change["change"]["actions"]
        print(f"  {change['address']}: {'+'.join(actions)}")
        if DESTRUCTIVE_ACTIONS.intersection(actions):
            destructive.append(f"{change['address']}: {'+'.join(actions)}")
        if change["address"].startswith("agentlink_"):
            leftover.append(change["address"])

    expected = {new for _, new, _ in MAPPING}
    missing = expected.difference(change["address"] for change in changes)

    if destructive:
        print("\nFAIL: migration must not create or destroy anything:", file=sys.stderr)
        for entry in destructive:
            print(f"  {entry}", file=sys.stderr)
        return 1
    if leftover:
        print(f"\nFAIL: agentlink resources still in state: {sorted(leftover)}", file=sys.stderr)
        return 1
    if missing:
        print(f"\nFAIL: resources absent from the plan: {sorted(missing)}", file=sys.stderr)
        return 1

    print("\nOK: every planned change is an in-place update")
    return 0


def cmd_snapshot(state_path):
    """Reduces `terraform show -json` to {address: {attribute: value}} for managed resources.

    Sensitive attributes are replaced with a digest rather than stored: a change still shows up, but
    the snapshot never holds a secret and a reported difference never prints one.
    """
    state = json.load(open(state_path))
    snapshot = {}
    for resource in state.get("values", {}).get("root_module", {}).get("resources", []):
        if resource.get("mode") != "managed":
            continue
        sensitive = resource.get("sensitive_values", {})
        snapshot[resource["address"]] = {
            attribute: digest(value) if sensitive.get(attribute) else value
            for attribute, value in resource.get("values", {}).items()
        }
    json.dump(snapshot, sys.stdout, indent=2, sort_keys=True)


def digest(value):
    encoded = json.dumps(value, sort_keys=True).encode()
    return f"sha256:{hashlib.sha256(encoded).hexdigest()[:16]}"


def cmd_assert_preserved(before_path, after_path):
    before = json.load(open(before_path))
    after = json.load(open(after_path))

    unexpected = []
    for address in sorted(before):
        allowed = EXPECTED_REWRITES.get(address, set())
        current = after.get(address)
        if current is None:
            unexpected.append(f"{address}: no longer in state")
            continue
        for attribute in sorted(before[address]):
            if attribute in allowed:
                continue
            was, now = before[address][attribute], current.get(attribute)
            if was != now:
                unexpected.append(f"{address}.{attribute}: {was!r} -> {now!r}")

    if unexpected:
        print(
            "FAIL: the first apply rewrote values that import had already read back:",
            file=sys.stderr,
        )
        for entry in unexpected:
            print(f"  {entry}", file=sys.stderr)
        return 1

    checked = sum(len(values) for values in before.values())
    print(f"OK: {checked} attributes across {len(before)} resources came through the apply unchanged")
    return 0


def cmd_assert_ids(before_path, after_path):
    before = load_outputs(before_path)
    after = load_outputs(after_path)

    changed = {key: (value, after.get(key)) for key, value in before.items() if after.get(key) != value}
    if changed:
        print("FAIL: these objects were replaced rather than adopted:", file=sys.stderr)
        for key, (was, now) in changed.items():
            print(f"  {key}: {was} -> {now}", file=sys.stderr)
        return 1

    print(f"OK: all {len(before)} object IDs survived the migration unchanged")
    return 0


def main(argv):
    command = argv[1] if len(argv) > 1 else ""
    if command == "old-addresses":
        cmd_old_addresses()
        return 0
    if command == "imports":
        cmd_imports(argv[2])
        return 0
    if command == "assert-plan":
        return cmd_assert_plan(argv[2])
    if command == "snapshot":
        cmd_snapshot(argv[2])
        return 0
    if command == "assert-preserved":
        return cmd_assert_preserved(argv[2], argv[3])
    if command == "assert-ids":
        return cmd_assert_ids(argv[2], argv[3])

    print(__doc__, file=sys.stderr)
    print(
        "usage: migrate.py {old-addresses | imports IDS | assert-plan PLAN | snapshot STATE |\n"
        "                   assert-preserved BEFORE AFTER | assert-ids BEFORE AFTER}",
        file=sys.stderr,
    )
    return 2


if __name__ == "__main__":
    sys.exit(main(sys.argv))
