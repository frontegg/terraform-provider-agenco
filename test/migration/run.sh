#!/usr/bin/env bash
#
# End-to-end test of the agentlink -> agenco migration documented in README.md.
#
#   ./run.sh              validate, create, migrate, verify, destroy
#   ./run.sh validate     parse both configs; no credentials, no API calls
#   ./run.sh create       stage 1: build the "before" state with frontegg/agentlink 0.4.7
#   ./run.sh migrate      stage 2: state rm + import into agenco
#   ./run.sh verify       stage 3: assert the plan is update-only, apply, assert convergence
#   ./run.sh destroy      stage 4: remove everything the test created
#
# create/migrate/verify/destroy write to the live Frontegg vendor the credentials point at.
# Use a non-production vendor.
set -euo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "$HERE/../.." && pwd)"
WORK="$HERE/workdir"
PLAN="$WORK/migration.tfplan"
PLAN_JSON="$WORK/migration-plan.json"
SNAPSHOT_IMPORTED="$WORK/snapshot-imported.json"
SNAPSHOT_APPLIED="$WORK/snapshot-applied.json"
IDS_BEFORE="$HERE/ids-agentlink.json"
IDS_AFTER="$HERE/ids-agenco.json"

# Points frontegg/agenco at this working copy for every stage. agentlink still resolves from the
# registry through the direct {} block, so stage 1 uses the released provider unmodified.
export TF_CLI_CONFIG_FILE="$ROOT/dev.tfrc"

step() { printf '\n\033[1m==> %s\033[0m\n' "$1"; }
die() {
  printf '\n\033[31mFAILED: %s\033[0m\n' "$1" >&2
  exit 1
}

require_credentials() {
  for name in FRONTEGG_CLIENT_ID FRONTEGG_SECRET FRONTEGG_REGION; do
    [ -n "${!name:-}" ] || die "$name is not set. Export it in your shell before running."
  done
}

build_provider() {
  step "Building the agenco provider from this working copy"
  make -C "$ROOT" build dev.tfrc
}

# create wipes the working directory, which would abandon whatever a previous run left behind. A
# stage that failed part-way through still has real objects recorded in state.
refuse_to_orphan_state() {
  [ -f "$WORK/terraform.tfstate" ] || return 0
  local tracked
  tracked=$(terraform -chdir="$WORK" state list 2>/dev/null | grep -cv '^data\.' || true)
  [ "$tracked" -eq 0 ] ||
    die "$tracked object(s) from a previous run are still in $WORK. Run './run.sh destroy' first."
}

seed_workdir() {
  cp "$HERE/configs/$1/main.tf" "$WORK/main.tf"
  cp "$HERE/openapi.json" "$WORK/openapi.json"
}

stage_validate() {
  build_provider
  for config in agentlink agenco; do
    step "Validating the $config configuration"
    local dir="$HERE/.validate/$config"
    rm -rf "$dir" && mkdir -p "$dir"
    cp "$HERE/configs/$config/main.tf" "$dir/main.tf"
    printf 'name_prefix = "validate"\n' >"$dir/terraform.tfvars"
    terraform -chdir="$dir" init -input=false -backend=false >/dev/null
    terraform -chdir="$dir" validate
  done
  rm -rf "$HERE/.validate"
}

stage_create() {
  require_credentials
  refuse_to_orphan_state
  build_provider
  step "Stage 1 — creating the migration fixture with frontegg/agentlink 0.4.7"

  rm -rf "$WORK" && mkdir -p "$WORK"
  seed_workdir agentlink
  # A per-run prefix keeps repeated runs from colliding. Later stages read it back from tfvars.
  printf 'name_prefix = "tf-migration-%s"\n' "$(date +%Y%m%d-%H%M%S)" >"$WORK/terraform.tfvars"

  terraform -chdir="$WORK" init -input=false
  terraform -chdir="$WORK" apply -auto-approve -input=false
  terraform -chdir="$WORK" output -json migration_ids >"$IDS_BEFORE"

  step "Stage 1 complete — recorded IDs in $(basename "$IDS_BEFORE")"
}

stage_migrate() {
  require_credentials
  build_provider
  [ -f "$IDS_BEFORE" ] || die "no recorded IDs; run './run.sh create' first"
  step "Stage 2 — switching the configuration to agenco"

  seed_workdir agenco

  # Written to files rather than piped: a failing helper has to abort the stage, and process
  # substitution would just feed the loop nothing and let it "succeed".
  local addresses="$WORK/old-addresses.txt"
  local imports="$WORK/imports.tsv"
  local present="$WORK/state-list.txt"
  python3 "$HERE/migrate.py" old-addresses >"$addresses"
  python3 "$HERE/migrate.py" imports "$IDS_BEFORE" >"$imports"
  [ -s "$imports" ] || die "no import list was produced from $(basename "$IDS_BEFORE")"
  terraform -chdir="$WORK" state list >"$present"

  step "Forgetting each agentlink resource (state rm touches nothing remotely)"
  while IFS= read -r address; do
    if grep -qxF "$address" "$present"; then
      terraform -chdir="$WORK" state rm "$address"
    else
      printf '  %s is already absent from state\n' "$address"
    fi
  done <"$addresses"

  step "Importing each object as its agenco type (import only reads)"
  while IFS=$'\t' read -r address id; do
    if grep -qxF "$address" "$present"; then
      printf '  %s is already in state\n' "$address"
      continue
    fi
    printf '  %s <- %s\n' "$address" "$id"
    terraform -chdir="$WORK" import -input=false "$address" "$id"
  done <"$imports"

  # What import managed to read back. Stage 3 asserts the first apply does not change any of it.
  terraform -chdir="$WORK" show -json >"$WORK/state-imported.json"
  python3 "$HERE/migrate.py" snapshot "$WORK/state-imported.json" >"$SNAPSHOT_IMPORTED"

  step "Stage 2 complete"
}

stage_verify() {
  require_credentials
  build_provider
  [ -d "$WORK" ] || die "no working directory; run './run.sh create migrate' first"
  step "Stage 3 — planning the first apply after migration"

  terraform -chdir="$WORK" plan -input=false -out="$PLAN"
  terraform -chdir="$WORK" show -json "$PLAN" >"$PLAN_JSON"

  step "Asserting the migration is state-only"
  python3 "$HERE/migrate.py" assert-plan "$PLAN_JSON" ||
    die "the migration plan would create or destroy an object"

  step "Applying the plan that was just asserted"
  terraform -chdir="$WORK" apply -input=false "$PLAN"

  step "Asserting the apply did not rewrite anything import had already read"
  [ -f "$SNAPSHOT_IMPORTED" ] || die "no post-import snapshot; re-run './run.sh migrate'"
  terraform -chdir="$WORK" show -json >"$WORK/state-applied.json"
  python3 "$HERE/migrate.py" snapshot "$WORK/state-applied.json" >"$SNAPSHOT_APPLIED"
  python3 "$HERE/migrate.py" assert-preserved "$SNAPSHOT_IMPORTED" "$SNAPSHOT_APPLIED" ||
    die "the first apply silently changed a live value"

  step "Asserting the configuration has converged"
  local exit_code=0
  terraform -chdir="$WORK" plan -input=false -detailed-exitcode || exit_code=$?
  case "$exit_code" in
  0) echo "OK: the second plan is empty" ;;
  2) die "the second plan still wants changes, so something does not round-trip" ;;
  *) die "terraform plan errored with exit code $exit_code" ;;
  esac

  step "Asserting every object was adopted rather than replaced"
  terraform -chdir="$WORK" output -json migration_ids >"$IDS_AFTER"
  python3 "$HERE/migrate.py" assert-ids "$IDS_BEFORE" "$IDS_AFTER" ||
    die "an object ID changed during the migration"

  step "Stage 3 complete — migration verified"
}

stage_destroy() {
  require_credentials
  [ -d "$WORK" ] || die "no working directory to destroy"
  step "Stage 4 — destroying the fixture"

  # Destroying an empty state reports success while leaving every object behind. If a stage failed
  # after state rm, that is exactly the situation, so say so instead of pretending to clean up.
  local tracked
  tracked=$(terraform -chdir="$WORK" state list 2>/dev/null | grep -cv '^data\.' || true)
  [ "$tracked" -gt 0 ] ||
    die "state tracks no objects, so there is nothing to destroy. If a run failed after 'state rm',
     the objects still exist in Frontegg. Recover their IDs from a workdir/terraform.tfstate*.backup
     into $(basename "$IDS_BEFORE"), then re-run './run.sh migrate' to adopt them."

  terraform -chdir="$WORK" destroy -auto-approve -input=false
  step "Stage 4 complete"
}

case "${1:-all}" in
validate) stage_validate ;;
create) stage_create ;;
migrate) stage_migrate ;;
verify) stage_verify ;;
destroy) stage_destroy ;;
all)
  stage_validate
  stage_create
  stage_migrate
  stage_verify
  stage_destroy
  step "All stages passed"
  ;;
*) die "unknown stage '$1'; expected validate, create, migrate, verify, destroy or all" ;;
esac
