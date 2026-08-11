# Terraform Provider for Frontegg Agenco

Manage Frontegg Agenco as code: applications, MCP gateway configuration and sources, tools,
connectors, prompts, access policies, and runtime governance controls.

This provider supersedes [`frontegg/agentlink`](https://registry.terraform.io/providers/frontegg/agentlink).
See [Migrating from agentlink](#migrating-from-agentlink).

```hcl
terraform {
  required_providers {
    agenco = {
      source  = "frontegg/agenco"
      version = "~> 1.0"
    }
  }
}

provider "agenco" {
  region = "eu" # client_id / secret come from FRONTEGG_CLIENT_ID / FRONTEGG_SECRET
}
```

## Authentication

The provider authenticates with vendor credentials from the Frontegg portal
(**⚙ Settings → API tokens**), exchanged at `POST /auth/vendor` for a vendor-scoped token.

| Setting | Argument | Environment variable |
| --- | --- | --- |
| Client ID | `client_id` | `FRONTEGG_CLIENT_ID` |
| API key | `secret` | `FRONTEGG_SECRET` |
| Region | `region` | `FRONTEGG_REGION` |
| Explicit base URL | `base_url` | `FRONTEGG_BASE_URL` |

Regions: `eu` (default), `us`, `au`, `ca`, `uk`, `stg`. `base_url` overrides `region` when both
are set.

Every resource in this provider is vendor-scoped. Tenant-scoped endpoints
(`/resources/policies/v1/tenant/*`, `/resources/security/v1/*/tenant`) require a user token with
tenant permissions and are deliberately **not** covered — use the `tenant_id` argument on the
policy resources to manage tenant-scoped policies with vendor credentials instead.

## Resources

### Application and MCP gateway

| Resource | Manages |
| --- | --- |
| `agenco_saas_app` | Whole-flow onboarding: application + hosted login + MCP configuration |
| `agenco_application` | The Frontegg application every other resource hangs off |
| `agenco_mcp_configuration` | Per-application MCP gateway settings, behavior risk, tool paging |
| `agenco_mcp_source` | One upstream a gateway pulls tools from (REST, GraphQL, MCP proxy, …) |
| `agenco_frontegg_tools_source` | Toggle for the built-in Frontegg tenant tools source |
| `agenco_source_tools_active_status` | Bulk enable/disable of every tool from one source |

### Tools

| Resource | Manages |
| --- | --- |
| `agenco_tools_import` | Import tools into a source from an OpenAPI or GraphQL document |
| `agenco_tool` | Name, description, activation and auth of one existing tool |
| `agenco_tool_hook` | `LIST_TOOLS` / `CALL_TOOL` pre-hook code |
| `agenco_custom_code_tool` | A tool whose body is code you supply |
| `agenco_custom_integration` | An installed connector instance built from an integration template |
| `agenco_prompt` | A prompt served to agents alongside the tools |

### Policies

| Resource | Manages |
| --- | --- |
| `agenco_conditional_policy` | if/then access rule: allow, deny, step up, or require approval |
| `agenco_rbac_policy` | Tool access granted to a set of roles or permissions |
| `agenco_masking_policy` | Response masking via 90 built-in PII detectors and custom regexes |
| `agenco_custom_masking_regex` | A vendor-defined masking pattern |

### Governance and runtime controls

| Resource | Manages |
| --- | --- |
| `agenco_ip_restriction` | One IP or CIDR entry |
| `agenco_ip_restriction_config` | Whether the IP list allows or blocks |
| `agenco_geo_fence` | Country list |
| `agenco_geo_fence_config` | Whether the country list allows or blocks |
| `agenco_time_of_work` | Working-hours windows and what happens outside them |
| `agenco_rate_limit_config` | Per-minute budgets by source, tool, tenant and user |
| `agenco_agent_type` | Which AI platforms may connect, and for which groups |
| `agenco_agent` | A registered agent identity and its issued credentials |

### Vendor-level singletons

| Resource | Manages |
| --- | --- |
| `agenco_allowed_origins` | Vendor CORS allow-list |
| `agenco_identity_configuration` | Default access-token lifetime |

Declare each of these at most once per vendor — they are not per-application.

## Data sources

| Data source | Returns |
| --- | --- |
| `agenco_application` | An application looked up by `id` or `name` |
| `agenco_tools` | An application's tools, optionally narrowed to one source |
| `agenco_integration_templates` | Connector templates available to your vendor |

## Onboarding an application

`agenco_saas_app` creates the application and its MCP gateway configuration together, so
onboarding an Agenco-enabled application is one resource instead of two — and a name is all it
needs:

```hcl
resource "agenco_saas_app" "orders_agent" {
  name = "Orders Agent"
}
```

Everything else the two APIs require is either derived or defaulted, matching what Frontegg's own
onboarding does:

| Left unset | What the provider does |
| --- | --- |
| `login_url` | Derives `https://{app_host}/oauth` — the application's own OAuth endpoint, where MCP clients authenticate |
| `app_url` | Derives `https://{app_host}/oauth/portal` |
| `base_url` | Sends `https://example.com`, which the gateway replaces per-source at invocation time |
| `api_timeout` | Sends `5000` |
| `allow_dcr` | Sends `true`, so MCP clients can register themselves |

Any value you do supply is used verbatim and never overridden, so point the application at your own
URLs when it has a browser UI:

```hcl
resource "agenco_saas_app" "storefront" {
  name      = "Storefront"
  app_url   = "https://storefront.example.com"
  login_url = "https://storefront.example.com/login"
}
```

Deriving a URL costs one extra API call: the applications API requires both URLs on create but only
assigns the host during that same call, so the provider creates with a placeholder and then
patches. Supply both and it is a single call.

Those `/oauth` paths come from portal and bootstrap behavior, not a documented contract. If
Frontegg moves them, `TestFronteggOAuthURLs` is the guard.

Two things to know before using this resource:

- **Do not also manage the same application with `agenco_application` or
  `agenco_mcp_configuration`.** The MCP configuration endpoint is an upsert keyed on the
  application, so two resources pointed at one application fight over it on every apply. Use one
  approach or the other per application.
- **Destroying it deletes the application**, and the MCP configuration with it, since the API has
  no delete route for the configuration alone.

If a create partially succeeds — application created, URL patch or MCP configuration rejected —
the application is written to state before the error is reported, so it is never orphaned outside
Terraform. The next apply completes it.

## A minimal end-to-end configuration

An application, a REST source, tools imported from an OpenAPI document, and a masking policy over
all of them:

```hcl
resource "agenco_saas_app" "storefront" {
  name = "Storefront"
}

resource "agenco_mcp_source" "orders_api" {
  application_id = agenco_saas_app.storefront.application_id
  name           = "Orders API"
  type           = "REST"
  source_url     = "https://api.example.com"
}

resource "agenco_tools_import" "orders_api" {
  application_id = agenco_saas_app.storefront.application_id
  source_id      = agenco_mcp_source.orders_api.id
  schema_file    = "${path.module}/schemas/orders-openapi.json"
  schema_type    = "openapi"
}

resource "agenco_masking_policy" "strip_pii" {
  name              = "Strip PII from tool responses"
  enabled           = true
  application_ids   = [agenco_saas_app.storefront.application_id]
  internal_tool_ids = [] # empty means every tool

  detectors = ["credit_card", "email_address", "phone_number", "us_ssn"]

  targeting {
    then {
      result = "mask"
    }
  }
}
```

Everything downstream hangs off `application_id`, so swapping the first resource for the
fine-grained pair changes nothing else. Do that when you want the application and its gateway
configuration to have independent lifecycles — destroying an `agenco_mcp_configuration` leaves the
application standing, whereas destroying an `agenco_saas_app` deletes it:

```hcl
resource "agenco_application" "storefront" {
  name      = "Storefront"
  app_url   = "https://storefront.example.com"
  login_url = "https://storefront.example.com/login"
}

resource "agenco_mcp_configuration" "storefront" {
  application_id = agenco_application.storefront.id
}
```

`agenco_application` maps one-to-one onto the applications API object and so keeps that API's
required fields, including both URLs; only `agenco_saas_app` derives them.

## Import

Resources scoped to an application import with a composite ID, because the API reads them per
application:

```bash
terraform import agenco_mcp_source.orders_api "$APP_ID/$SOURCE_ID"
terraform import agenco_prompt.playbook "$APP_ID/$PROMPT_ID"
```

Singletons per application import with just the application ID:

```bash
terraform import agenco_mcp_configuration.storefront "$APP_ID"
terraform import agenco_geo_fence.eu_only "$APP_ID"
```

Resources keyed on a discriminator rather than a UUID use that value:

```bash
terraform import agenco_tool_hook.list_filter "$APP_ID/LIST_TOOLS"
terraform import agenco_agent_type.claude_code "$APP_ID/claude-code"
```

Each resource's page documents its exact import syntax.

## SaaS and Workforce

This provider currently manages **SaaS** MCP configurations. `agenco_mcp_configuration` sends
`type = "saas"` and does not expose the Workforce-only settings (`default_access`, which the
gateway only consults on the Workforce policy path).

The distinction is a single immutable field on the API's app-MCP-configuration object, so
Workforce support is additive: it will arrive as a separate resource rather than as a breaking
change here. Everything else — sources, tools, policies, governance — is keyed on the application
and behaves identically for both.

## Known API constraints

These are limits of the underlying Frontegg API, surfaced here so the behavior is not surprising:

- **`agenco_mcp_configuration` cannot be deleted.** The API has no delete route. Destroying the
  resource removes it from state and warns; deleting the application removes the configuration.
- **`agenco_identity_configuration` cannot be deleted**, for the same reason.
- **`agenco_agent` cannot be updated.** The agent registry has create and delete only, so every
  configurable attribute forces replacement — which rotates the agent's credentials.
- **`agenco_rbac_policy` does not detect drift in `enabled` or `application_ids`.** The API's RBAC
  read route projects only the shared policy fields plus `keys`, so those two attributes keep
  their configured values.
- **Write-only fields keep their configured values**: `client_secret` and `api_key` are returned
  masked or omitted, `code`/`code_content` live in the code-execution service, and
  `input_schema`/`output_schema` are folded into a larger server-side schema object.
- **`agenco_source_tools_active_status` has no readable state.** It wraps a bulk action; there is
  no route returning the aggregate status, so its read is a no-op and destroy does nothing.
- **`agenco_time_of_work` window offsets are milliseconds from midnight**, and a weekday may
  appear in only one window across the whole rule.
- **`agenco_saas_app` spans two API objects.** Destroying it deletes the application, and it must
  not be combined with `agenco_application` / `agenco_mcp_configuration` for the same application.
- **`allow_cimd` and `dpop_enforcement_type` on `agenco_application` are read-only.** The
  applications API returns both, but its documented create and update payloads do not accept
  them, so the provider reports rather than manages them.

## Defaults

Where the API requires a value the provider has to supply one. These were taken from a portal
SaaS-onboarding trace so that a Terraform-created application matches a portal-created one:

| Attribute | Default | Source |
| --- | --- | --- |
| `agenco_mcp_configuration.api_timeout` | `5000` | What the portal sends; the API rejects a missing value |
| `base_url` (both MCP resources) | `https://example.com` | The portal's placeholder; overridden per-source at runtime |
| `agenco_mcp_source.api_timeout` | `5000` | Kept consistent with the configuration above |
| `agenco_mcp_configuration.behavior_risk_threshold` | `high` | Matches the server-side default |
| `enable_advanced_tools`, `slim_semantic_search_enabled`, `integration_tools_enabled` | `false` | Match the server-side defaults |
| `behavior_risk_actions`, `list_tool_page_size` | unset | The API stores null when omitted |
| `agenco_application.type` | `web` | Documented API default |
| `agenco_application.frontend_stack` | `react` | Documented API default |
| `agenco_application.is_active` | `true` | Documented API default |
| `agenco_application.is_default` | `false` | Documented API default |
| `agenco_application.allow_dcr` | `true` | The state portal onboarding leaves an application in |
| `agenco_application.access_type` | `FREE_ACCESS` | What the portal sends on create |

These are declared as provider defaults rather than left to the API on purpose. An
optional-and-computed attribute with no default keeps its prior value when you delete it from the
configuration, so a value set once could never be reverted by removing the line. With a default,
removing the attribute reverts it.

`allow_dcr` is the one default that deliberately **differs** from the API's own: the API defaults
it to `false`, but portal onboarding switches it to `true` immediately after creating the
application, and Dynamic Client Registration is how MCP clients register themselves. The provider
follows the onboarding outcome. Set `allow_dcr = false` explicitly if you require pre-registered
OAuth clients.

Because the configuration is the source of truth, importing an existing application that differs
from these defaults will show the difference as a diff on the next plan — for example a
`MANAGED_ACCESS` application imported without an explicit `access_type` plans as
`MANAGED_ACCESS → FREE_ACCESS`. Write out the attributes you care about after importing.

Two notes on `access_type` specifically: `MANAGED_ACCESS` requires tenants to be assigned to the
application explicitly via `/resources/applications/tenant-assignments/v1`, which this provider
does **not** cover, so those assignments must be managed elsewhere. And it is the only optional
application attribute the API does not document a default for — `FREE_ACCESS` here comes from
observed portal behavior rather than the schema.

`base_url` deserves a note. The API requires it, but for any tool imported into an
`agenco_mcp_source` the gateway replaces it with that source's `source_url` at invocation time
(`tool-locator.service.ts`), so it only ever applies to tools upserted without a source. Set it if
you have those; otherwise the placeholder is harmless — `example.com` is reserved by RFC 2606 and
cannot reach a real service.

Note the API's own `apiTimeout` fallback constant is `3000`, but it is unreachable — the request
validator rejects a missing value outright, so the provider's default is what actually applies.

## Migrating from agentlink

`frontegg/agentlink` remains published and functional. To move a configuration across:

1. Update the provider requirement:

   ```hcl
   terraform {
     required_providers {
       agenco = {
         source  = "frontegg/agenco"
         version = "~> 1.0"
       }
     }
   }
   ```

2. Move existing state to the new provider:

   ```bash
   terraform state replace-provider registry.terraform.io/frontegg/agentlink \
                                    registry.terraform.io/frontegg/agenco
   ```

3. Rename the resource types in your configuration and state. The type prefix changes from
   `agentlink_` to `agenco_`, and two resources were renamed:

   | agentlink | agenco |
   | --- | --- |
   | `agentlink_application` | `agenco_application` |
   | `agentlink_mcp_configuration` | `agenco_mcp_configuration` |
   | `agentlink_source` | `agenco_mcp_source` |
   | `agentlink_tools_import` | `agenco_tools_import` |
   | `agentlink_conditional_policy` | `agenco_conditional_policy` |
   | `agentlink_rbac_policy` | `agenco_rbac_policy` |
   | `agentlink_masking_policy` | `agenco_masking_policy` |
   | `agentlink_allowed_origins` | `agenco_allowed_origins` |
   | `agentlink_identity_configuration` | `agenco_identity_configuration` |

   Use `terraform state mv` for each resource, then `terraform plan` to confirm no changes.

4. Adjust for the schema changes:

   - **`agenco_masking_policy`** replaces the 15 boolean detector attributes with a single
     `detectors` set covering all 90 detectors. `credit_card = true` becomes
     `detectors = ["credit_card"]`.
   - **Policy `app_ids`** is now `application_ids`.
   - **`agenco_mcp_source`** gained `is_local`, `slug`, `two_step_callback`, `override_headers`,
     `external_authorization_url`, `scopes`, `client_id` and `client_secret`. The old `vendor_id`
     attribute is now computed only.
   - **Policy `targeting`** is a real nested block supporting nested `condition_group`s, rather
     than a flat object.

## Development

```bash
make build      # compile the provider
make test       # unit tests, including provider schema validation
make testacc    # acceptance tests (requires TF_ACC and live credentials)
make fmt vet    # format and vet
make docs       # regenerate docs/ from the schemas and examples/
```

Documentation under `docs/` is generated by
[tfplugindocs](https://github.com/hashicorp/terraform-plugin-docs) from the resource schemas and
the `examples/` tree. Edit schema descriptions or examples, then regenerate — do not hand-edit
`docs/`. `templates/` holds the one custom page template (the masking detector table).

To try the provider against a live account before it is published, build it and point Terraform
at the local binary with a `dev_overrides` block in a `.tfrc` file.
