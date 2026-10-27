-- Service DB (proposal / notification / analytics): same pattern as 01_customer_db.sql (D-019),
-- plus a schema named after the service, owned by the service role (D-048).
-- Run as superuser on the default DB, once per service:  -v svc=<name> -v db_password=<secret>
-- No tables here: they come from the service's own goose migrations (D-020).
\set role :svc '_svc'
\set db :svc '_db'

CREATE ROLE :"role" LOGIN PASSWORD :'db_password';

CREATE DATABASE :"db" OWNER :"role" ENCODING 'UTF8' TEMPLATE template0;

-- Service boundary: no other role may even connect.
REVOKE ALL ON DATABASE :"db" FROM PUBLIC;
GRANT CONNECT, TEMPORARY ON DATABASE :"db" TO :"role";

-- Tables live in the service-named schema; goose's version table lands there too.
ALTER ROLE :"role" IN DATABASE :"db" SET search_path = :"svc", public;

\connect :"db"
CREATE SCHEMA :"svc" AUTHORIZATION :"role";
