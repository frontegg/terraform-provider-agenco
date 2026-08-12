# Adopts an import that already happened, without calling the API.
terraform import agenco_tools_import.orders_api 0a1b2c3d-4e5f-6789-abcd-ef0123456789/11112222-3333-4444-5555-666677778888

# schema_file and schema_type are not recoverable from the API — nothing records which document
# produced a tool — so add both to your configuration before importing. schema_hash stays unset,
# so the next apply re-imports the document and upserts the tools: that rewrites tool definitions
# for the source, but creates nothing new and deletes nothing.
