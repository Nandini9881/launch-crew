-- +goose Up
CREATE TABLE users (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE buffer_connections (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    buffer_account_id text NOT NULL UNIQUE,
    org_id text,
    access_token_enc bytea NOT NULL,
    refresh_token_enc bytea,
    key_id text NOT NULL,
    expires_at timestamptz,
    status text NOT NULL DEFAULT 'ok' CHECK (status IN ('ok', 'needs_reconnect')),
    channels_json jsonb,
    channels_cached_at timestamptz,
    config_json jsonb,
    config_cached_at timestamptz,
    rate_remaining int,
    rate_reset_at timestamptz
);
CREATE INDEX buffer_connections_user_idx ON buffer_connections (user_id);

CREATE TABLE github_identities (
    github_user_id bigint PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    login text NOT NULL
);

CREATE TABLE opt_outs (
    github_user_id bigint PRIMARY KEY,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE repos (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    installation_id bigint NOT NULL UNIQUE,
    owner_user_id uuid REFERENCES users (id) ON DELETE CASCADE,
    full_name text NOT NULL,
    enabled bool NOT NULL DEFAULT true,
    allow_prerelease bool NOT NULL DEFAULT false
);

CREATE TABLE launches (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_user_id uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    repo_id uuid REFERENCES repos (id) ON DELETE SET NULL,
    source text NOT NULL,
    status text NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'launched', 'closed', 'deleted')),
    title text NOT NULL DEFAULT '',
    summary text NOT NULL DEFAULT '',
    release_tag text,
    release_url text,
    headline_changes jsonb NOT NULL DEFAULT '[]',
    roles jsonb NOT NULL DEFAULT '[]',
    launch_at timestamptz,
    window_minutes int NOT NULL DEFAULT 90,
    open_token_hash bytea UNIQUE,
    crew_cap int NOT NULL DEFAULT 200,
    card_preset smallint,
    hero_asset_id uuid,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX launches_owner_idx ON launches (owner_user_id, created_at DESC);

CREATE TABLE launch_assets (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    launch_id uuid NOT NULL REFERENCES launches (id) ON DELETE CASCADE,
    kind text NOT NULL,
    r2_url text NOT NULL,
    sha text NOT NULL
);
CREATE INDEX launch_assets_launch_idx ON launch_assets (launch_id);

CREATE TABLE captain_drafts (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    launch_id uuid NOT NULL REFERENCES launches (id) ON DELETE CASCADE,
    channel_id text NOT NULL,
    text text NOT NULL DEFAULT '',
    thread jsonb,
    first_comment text,
    asset_id uuid,
    status text NOT NULL DEFAULT 'draft',
    buffer_post_id text,
    UNIQUE (launch_id, channel_id)
);

CREATE TABLE voice_profiles (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    buffer_account_id text NOT NULL UNIQUE,
    profile jsonb NOT NULL,
    derived_from text NOT NULL DEFAULT '',
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE crew_members (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    launch_id uuid NOT NULL REFERENCES launches (id) ON DELETE CASCADE,
    user_id uuid REFERENCES users (id) ON DELETE SET NULL,
    buffer_account_id text,
    github_user_id bigint,
    role text NOT NULL DEFAULT '',
    role_hint text NOT NULL DEFAULT '',
    talk_title text NOT NULL DEFAULT '',
    invite_token_hash bytea UNIQUE,
    state text NOT NULL DEFAULT 'invited',
    pr_number int,
    pr_title text,
    pr_comment_id bigint,
    voice_profile_id uuid REFERENCES voice_profiles (id) ON DELETE SET NULL,
    neutral_draft text,
    card_asset_id uuid,
    opened_at timestamptz,
    token_expires_at timestamptz,
    UNIQUE (launch_id, buffer_account_id),
    UNIQUE (launch_id, github_user_id)
);
CREATE INDEX crew_members_launch_idx ON crew_members (launch_id);

CREATE TABLE crew_drafts (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    crew_member_id uuid NOT NULL REFERENCES crew_members (id) ON DELETE CASCADE,
    channel_id text NOT NULL,
    network text NOT NULL,
    text text NOT NULL DEFAULT '',
    thread jsonb,
    first_comment text,
    asset_id uuid,
    hook_type text,
    overlap_score real,
    flagged bool NOT NULL DEFAULT false,
    status text NOT NULL DEFAULT 'draft',
    buffer_post_id text,
    due_at timestamptz,
    edited bool NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (crew_member_id, channel_id)
);

CREATE TABLE post_attempts (
    crew_member_id uuid NOT NULL REFERENCES crew_members (id) ON DELETE CASCADE,
    channel_id text NOT NULL,
    status text NOT NULL,
    buffer_post_id text,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (crew_member_id, channel_id)
);

CREATE TABLE buffer_calls (
    id bigserial PRIMARY KEY,
    launch_id uuid REFERENCES launches (id) ON DELETE SET NULL,
    connection_id uuid NOT NULL REFERENCES buffer_connections (id) ON DELETE CASCADE,
    op text NOT NULL,
    status int NOT NULL,
    request_id text,
    rate_remaining int,
    at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX buffer_calls_conn_at_idx ON buffer_calls (connection_id, at);

CREATE TABLE webhook_deliveries (
    delivery_id text PRIMARY KEY,
    event text NOT NULL,
    payload jsonb NOT NULL,
    received_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE jobs (
    id bigserial PRIMARY KEY,
    kind text NOT NULL,
    payload jsonb NOT NULL DEFAULT '{}',
    run_at timestamptz NOT NULL DEFAULT now(),
    attempts int NOT NULL DEFAULT 0,
    locked_at timestamptz,
    locked_by text,
    last_error text,
    dead bool NOT NULL DEFAULT false,
    launch_id uuid REFERENCES launches (id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX jobs_ready_idx ON jobs (dead, run_at) WHERE locked_at IS NULL;

CREATE TABLE open_rate (
    launch_id uuid NOT NULL REFERENCES launches (id) ON DELETE CASCADE,
    ip inet NOT NULL,
    minute timestamptz NOT NULL,
    n int NOT NULL DEFAULT 1,
    PRIMARY KEY (launch_id, ip, minute)
);

-- +goose Down
DROP TABLE open_rate, jobs, webhook_deliveries, buffer_calls, post_attempts, crew_drafts,
    crew_members, voice_profiles, captain_drafts, launch_assets, launches, repos, opt_outs,
    github_identities, buffer_connections, users;
