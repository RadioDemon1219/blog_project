-- +goose Up
alter table posts add column  status text not null default 'published';

-- +goose Down
alter table posts drop column status;