# agentlink → agenco migration test

Executes the migration described in the [root README](../../README.md#migrating-from-agentlink)
against a live Frontegg vendor and asserts the claim the guide makes to users: **migration is
state-only — no Frontegg object is destroyed or recreated.**

The test proves that four ways:

1. The plan produced immediately after `state rm` + `import` contains no `create` and no `delete`.
2. The second plan, after applying the first, is empty — so everything round-trips.
3. Every object ID recorded before the migration is still in state afterwards.
4. Every attribute value that `import` read back survives the first apply unchanged, except for a
   short allow-list of fields the API genuinely cannot return (`migrate.py:EXPECTED_REWRITES`).

Assertion 4 exists because the first three do not catch a silently changed value: rewriting a live
attribute plans as an in-place update, and converges cleanly on the next plan. Sensitive attributes
are compared by digest, so a rotated secret is still detected without the value being stored or
printed.

## Running it

```bash
export FRONTEGG_REGION=stg
export FRONTEGG_CLIENT_ID=...       # vendor client ID
export FRONTEGG_SECRET=...          # vendor secret

./run.sh
```

`./run.sh` runs every stage and cleans up. Each stage is also individually runnable, which is what
you want when something fails partway:

| Stage | What it does | Touches the API |
| --- | --- | --- |
| `validate` | Parses both configs against the real provider schemas | no |
| `create` | Builds the "before" state with `frontegg/agentlink` 0.4.7 from the registry | yes |
| `migrate` | `state rm` each agentlink resource, `import` each agenco resource | yes (reads) |
| `verify` | Asserts the plan, applies it, asserts convergence and IDs | yes |
| `destroy` | Removes everything the test created | yes |

`validate` needs no credentials and is worth running after any schema change.

If a live stage fails part-way, real objects are already recorded in `workdir/`. Run
`./run.sh destroy` before `create` again — `create` refuses to wipe a working directory that still
tracks objects, rather than abandoning them.

The agenco provider is taken from this working copy through `dev.tfrc`, so nothing has to be
published to test a migration into it. agentlink comes from the registry, unmodified — the test
migrates from the version customers actually have.

Everything the test creates is prefixed `tf-migration-<timestamp>`, so repeated runs do not
collide. Point the credentials at a non-production vendor.

## What is covered

Seven resources, one per migrating type, mapped in `migrate.py`:

| agentlink | agenco | Import ID |
| --- | --- | --- |
| `agentlink_application` | `agenco_application` | application ID |
| `agentlink_mcp_configuration` | `agenco_mcp_configuration` | application ID |
| `agentlink_source` | `agenco_mcp_source` | `app_id/source_id` |
| `agentlink_tools_import` | `agenco_tools_import` | `app_id/source_id` |
| `agentlink_masking_policy` | `agenco_masking_policy` | policy ID |
| `agentlink_rbac_policy` | `agenco_rbac_policy` | policy ID |
| `agentlink_conditional_policy` | `agenco_conditional_policy` | policy ID |

The interesting schema changes are exercised rather than avoided: `app_ids` → `application_ids`,
the fifteen masking detector booleans → one `detectors` set, `agentlink_source` → the renamed and
extended `agenco_mcp_source`, and the two defaults that differ between the providers.

The RBAC policy targets a real tool rather than an empty list, because agentlink rejects an empty
`internal_tool_ids` for RBAC policies (`resource_rbac_policy.go:148`). agentlink has no data source
for tools, so both configs read the IDs through `agenco_tools` — the only thing the agenco provider
does during stage 1. The selection is `sort(...)[0]` rather than `[0]`, so both stages pick the same
tool regardless of the order the API returns them in.

Worth noting separately: **agenco does not have that guard.** It will happily create an RBAC policy
with no tools selected. Either the API accepts it and agentlink was over-validating, or agenco
should re-add the check.

The agenco config deliberately omits `type`, `allow_dcr`, `frontend_stack`, `access_type`,
`is_active` and `is_default`, all of which the agentlink stage sets explicitly. agenco's defaults for
`type` and `allow_dcr` are the *opposite* of the resulting live values, so if any provider default
were applied on update rather than only on create, the imported application would be rewritten.

To confirm assertion 4 is load-bearing, swap one `createDefault*` plan modifier back to a
schema-level `Default` and re-run: the plan stays update-only and the second plan still converges, so
assertions 1–3 pass, and only assertion 4 catches it.

## What is deliberately not covered

**`agentlink_allowed_origins` and `agentlink_identity_configuration`** are vendor-wide singletons.
Creating them overwrites settings shared by every application in the vendor, and destroying
`allowed_origins` clears the entire CORS allow-list. Migrating them is a plain
`ImportStatePassthroughID`, and exercising it automatically is not worth the risk to a shared
account. Migrate them by hand.

**Policy `targeting`.** agentlink accepts a `targeting` attribute on `conditional_policy` and
`masking_policy` but discards it —
[`buildTargetingFromState`](https://github.com/frontegg/terraform-provider-agentlink/blob/master/internal/provider/resource_conditional_policy.go)
returns `nil` unconditionally. There is therefore never any targeting on a live object for the
migration to preserve, and nothing to assert. Worth knowing separately: a user who had `targeting`
in their agentlink configuration will find it starts taking effect under agenco.
