-- +goose Up
create table posts(
    id bigserial primary key,
    title text not null,
    body text not null,
    created_at timestamptz not null default now()
);

-- +goose Down
drop table posts;

