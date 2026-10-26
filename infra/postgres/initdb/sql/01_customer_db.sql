-- customer_db: owned by the customer service role only. Run as superuser on the default DB.
CREATE ROLE customer_svc LOGIN PASSWORD :'customer_db_password';

CREATE DATABASE customer_db OWNER customer_svc ENCODING 'UTF8' TEMPLATE template0;

-- Service boundary: no other role may even connect.
REVOKE ALL ON DATABASE customer_db FROM PUBLIC;
GRANT CONNECT, TEMPORARY ON DATABASE customer_db TO customer_svc;

ALTER ROLE customer_svc IN DATABASE customer_db SET search_path = dwh, public;
