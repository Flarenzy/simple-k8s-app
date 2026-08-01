-- +goose Up
CREATE TABLE kubernetes_sources (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    source_key TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    site_id UUID NOT NULL REFERENCES sites(id) ON DELETE CASCADE,
    cluster_domain TEXT NOT NULL,
    namespace_scope TEXT[] NOT NULL,
    last_attempt_at TIMESTAMPTZ,
    last_success_at TIMESTAMPTZ,
    last_error TEXT NOT NULL DEFAULT '',
    service_count INTEGER NOT NULL DEFAULT 0,
    matched_count INTEGER NOT NULL DEFAULT 0,
    unmatched_count INTEGER NOT NULL DEFAULT 0,
    ambiguous_count INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE kubernetes_services (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    source_id UUID NOT NULL REFERENCES kubernetes_sources(id) ON DELETE CASCADE,
    kubernetes_uid TEXT NOT NULL,
    namespace TEXT NOT NULL,
    name TEXT NOT NULL,
    service_type TEXT NOT NULL,
    resource_version TEXT NOT NULL,
    external_name TEXT NOT NULL DEFAULT '',
    dns_name TEXT NOT NULL,
    observed_at TIMESTAMPTZ NOT NULL,
    active BOOLEAN NOT NULL DEFAULT true,
    stale_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT kubernetes_services_source_uid_unique UNIQUE (source_id, kubernetes_uid)
);

CREATE INDEX kubernetes_services_source_name_idx
    ON kubernetes_services (source_id, namespace, name);

CREATE TABLE kubernetes_service_ports (
    id BIGSERIAL PRIMARY KEY,
    service_id UUID NOT NULL REFERENCES kubernetes_services(id) ON DELETE CASCADE,
    name TEXT,
    protocol TEXT NOT NULL,
    port INTEGER NOT NULL,
    target_port TEXT NOT NULL,
    app_protocol TEXT,
    node_port INTEGER
);

CREATE TABLE kubernetes_service_addresses (
    id BIGSERIAL PRIMARY KEY,
    service_id UUID NOT NULL REFERENCES kubernetes_services(id) ON DELETE CASCADE,
    kind TEXT NOT NULL CHECK (kind IN ('cluster_ip', 'load_balancer')),
    address INET NOT NULL,
    ip_mode TEXT,
    ip_address_id UUID REFERENCES ip_addresses(id) ON DELETE SET NULL,
    match_status TEXT NOT NULL CHECK (match_status IN ('matched', 'unmatched', 'ambiguous')),
    match_count INTEGER NOT NULL,
    CONSTRAINT kubernetes_service_addresses_unique UNIQUE (service_id, kind, address)
);

CREATE INDEX kubernetes_service_addresses_ip_idx
    ON kubernetes_service_addresses (ip_address_id)
    WHERE ip_address_id IS NOT NULL;

CREATE TABLE kubernetes_service_hostnames (
    id BIGSERIAL PRIMARY KEY,
    service_id UUID NOT NULL REFERENCES kubernetes_services(id) ON DELETE CASCADE,
    kind TEXT NOT NULL CHECK (kind = 'load_balancer'),
    hostname TEXT NOT NULL,
    CONSTRAINT kubernetes_service_hostnames_unique UNIQUE (service_id, kind, hostname)
);

-- +goose Down
DROP TABLE kubernetes_service_hostnames;
DROP TABLE kubernetes_service_addresses;
DROP TABLE kubernetes_service_ports;
DROP TABLE kubernetes_services;
DROP TABLE kubernetes_sources;
