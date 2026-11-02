-- +goose Up
ALTER TABLE repos DROP CONSTRAINT IF EXISTS repos_installation_id_key;
ALTER TABLE repos ADD CONSTRAINT repos_installation_full_name_key UNIQUE (installation_id, full_name);
ALTER TABLE repos ADD COLUMN github_repo_id bigint;

-- +goose Down
ALTER TABLE repos DROP COLUMN github_repo_id;
ALTER TABLE repos DROP CONSTRAINT repos_installation_full_name_key;
ALTER TABLE repos ADD CONSTRAINT repos_installation_id_key UNIQUE (installation_id);
