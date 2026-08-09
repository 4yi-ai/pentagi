-- +goose Up
-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS termlogs_flow_created_at_idx
  ON termlogs(flow_id, created_at, id);
CREATE INDEX IF NOT EXISTS assistantlogs_flow_assistant_created_at_idx
  ON assistantlogs(flow_id, assistant_id, created_at, id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS assistantlogs_flow_assistant_created_at_idx;
DROP INDEX IF EXISTS termlogs_flow_created_at_idx;
-- +goose StatementEnd
