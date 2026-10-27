-- Seed customer_db.dwh from the read-only stub mount (/seed/stub).
-- Each CSV lands in a session-only TEMP table whose column names equal the CSV header,
-- so HEADER MATCH aborts the load if a stub file's columns change. TEMP tables are not
-- WAL-logged and vanish with the session: card, CVV and IBAN never reach a durable table.
-- D-017: time columns shift from AS_OF 2026-10-26 12:00 UTC to load time (timestamptz by interval, date by whole days).
\set ON_ERROR_STOP on
BEGIN;
SET LOCAL search_path = dwh;

-- D-017: shift generated timestamps (AS_OF = 2026-10-26 12:00 UTC) to load time.
-- now() is the transaction start time, so every row gets the same shift.
SELECT now() - TIMESTAMPTZ '2026-10-26 12:00:00Z' AS ts_shift,
       (now() AT TIME ZONE 'UTC')::date - DATE '2026-10-26' AS day_shift
\gset

-- ------------------------------------------------------------------ staging
CREATE TEMP TABLE stg_user (
    "Customer ID" text, "Address Street" text, "Address City" text, "Address Postcode" text,
    "Age (Years)" text, "Distance Commuted to Work (Km)" text, "Employer Company" text,
    "Credit Card Start Date" text, "Credit Card Expiry Date" text, "Credit Card Number" text,
    "Credit Card CVV" text, "Dependants" text, "First Name" text, "Bank IBAN" text, "Last Name" text,
    "Marital Status" text, "Yearly Pension (GBP)" text, "Retired" text, "Yearly Salary (GBP)" text,
    "Sex" text, "Vehicle Make" text, "Vehicle Model" text, "Vehicle Year" text, "Vehicle Type" text,
    "Email" text, "Email Bounced" text, "Phone (E.164)" text, "Country Code" text, "City" text,
    "Language" text, "Timezone" text, "Date of Birth" text, "Signup Date" text, "Account Status" text,
    "Deleted At" text, "Email Opt In" text, "SMS Opt In" text, "Consent Updated At" text,
    "Preferred Platform" text, "Total Orders" text, "Total Spend (GBP)" text,
    "Average Order Value (GBP)" text, "First Order Date" text, "Last Order Date" text,
    "Favourite Category" text, "Last Cart Add Date" text, "Has Open Cart" text,
    "Open Cart Value (GBP)" text, "Last Active Date" text, "Sessions (90d)" text, "Loyalty Tier" text,
    "Loyalty Points" text, "Campaigns Received (90d)" text, "Emails Opened (90d)" text,
    "Last Campaign Sent At" text
) ON COMMIT DROP;
COPY stg_user FROM '/seed/stub/acw_user_data.csv' WITH (FORMAT csv, HEADER MATCH, ENCODING 'UTF8');

CREATE TEMP TABLE stg_product (
    "Product ID" text, "Product Name" text, "Brand" text, "Category" text, "Price (GBP)" text,
    "In Stock" text, "Rating" text
) ON COMMIT DROP;
COPY stg_product FROM '/seed/stub/acw_products.csv' WITH (FORMAT csv, HEADER MATCH, ENCODING 'UTF8');

CREATE TEMP TABLE stg_order (
    "Order ID" text, "Customer ID" text, "Order Date" text, "Order Status" text, "Items Count" text,
    "Subtotal (GBP)" text, "Discount Code" text, "Discount (GBP)" text, "Shipping (GBP)" text,
    "Total (GBP)" text, "Payment Method" text, "Platform" text, "Shipping City" text, "Country Code" text
) ON COMMIT DROP;
COPY stg_order FROM '/seed/stub/acw_orders.csv' WITH (FORMAT csv, HEADER MATCH, ENCODING 'UTF8');

-- row_no keeps file order so line_no is stable for repeated products within an order
CREATE TEMP TABLE stg_order_item (
    row_no bigint GENERATED ALWAYS AS IDENTITY,
    "Order ID" text, "Product ID" text, "Quantity" text, "Unit Price (GBP)" text, "Line Total (GBP)" text
) ON COMMIT DROP;
COPY stg_order_item ("Order ID", "Product ID", "Quantity", "Unit Price (GBP)", "Line Total (GBP)")
    FROM '/seed/stub/acw_order_items.csv' WITH (FORMAT csv, HEADER MATCH, ENCODING 'UTF8');

CREATE TEMP TABLE stg_cart_item (
    "Cart Item ID" text, "Customer ID" text, "Product ID" text, "Quantity" text, "Unit Price (GBP)" text,
    "Added At" text, "Cart Status" text, "Converted At" text, "Order ID" text
) ON COMMIT DROP;
COPY stg_cart_item FROM '/seed/stub/acw_cart_items.csv' WITH (FORMAT csv, HEADER MATCH, ENCODING 'UTF8');

CREATE TEMP TABLE stg_event (
    "Event ID" text, "Customer ID" text, "Session ID" text, "Event Type" text, "Event Time" text,
    "Platform" text, "Product ID" text, "Category" text, "Value" text, "Order ID" text
) ON COMMIT DROP;
COPY stg_event FROM '/seed/stub/acw_events.csv' WITH (FORMAT csv, HEADER MATCH, ENCODING 'UTF8');

CREATE TEMP TABLE stg_consent (
    "Consent ID" text, "Customer ID" text, "Channel" text, "Consent Status" text, "Source" text,
    "Changed At" text
) ON COMMIT DROP;
COPY stg_consent FROM '/seed/stub/acw_consents.csv' WITH (FORMAT csv, HEADER MATCH, ENCODING 'UTF8');

-- ------------------------------------------------------------------ load (parents first)
INSERT INTO dim_customer (
    customer_id, first_name, last_name, sex, date_of_birth, age_years, marital_status, dependants,
    email, email_bounced, phone_e164, country_code, city, language, timezone,
    address_street, address_city, address_postcode,
    signup_date, account_status, deleted_at, last_active_date, preferred_platform,
    email_opt_in, sms_opt_in, consent_updated_at,
    total_orders, total_spend_gbp, avg_order_value_gbp, first_order_date, last_order_date,
    favourite_category, last_cart_add_date, has_open_cart, open_cart_value_gbp, sessions_90d,
    loyalty_tier, loyalty_points, campaigns_received_90d, emails_opened_90d, last_campaign_sent_at,
    employer_company, yearly_salary_gbp, yearly_pension_gbp, retired, commute_distance_km,
    vehicle_make, vehicle_model, vehicle_year, vehicle_type)
SELECT
    "Customer ID", "First Name", "Last Name", "Sex", "Date of Birth"::date + :day_shift, "Age (Years)"::smallint,
    "Marital Status", NULLIF("Dependants", '')::smallint,
    "Email", "Email Bounced"::boolean, NULLIF("Phone (E.164)", ''), "Country Code", "City", "Language", "Timezone",
    "Address Street", "Address City", "Address Postcode",
    "Signup Date"::date + :day_shift, "Account Status", NULLIF("Deleted At", '')::timestamptz + :'ts_shift'::interval,
    "Last Active Date"::date + :day_shift,
    "Preferred Platform",
    "Email Opt In"::boolean, "SMS Opt In"::boolean, "Consent Updated At"::timestamptz + :'ts_shift'::interval,
    "Total Orders"::integer, "Total Spend (GBP)"::numeric, "Average Order Value (GBP)"::numeric,
    NULLIF("First Order Date", '')::date + :day_shift, NULLIF("Last Order Date", '')::date + :day_shift,
    NULLIF("Favourite Category", ''), NULLIF("Last Cart Add Date", '')::date + :day_shift, "Has Open Cart"::boolean,
    "Open Cart Value (GBP)"::numeric, "Sessions (90d)"::integer,
    "Loyalty Tier", "Loyalty Points"::integer, "Campaigns Received (90d)"::integer,
    "Emails Opened (90d)"::integer, NULLIF("Last Campaign Sent At", '')::timestamptz + :'ts_shift'::interval,
    NULLIF(NULLIF("Employer Company", ''), 'N/A'), NULLIF("Yearly Salary (GBP)", '')::integer,
    NULLIF("Yearly Pension (GBP)", '')::integer, "Retired"::boolean,
    NULLIF("Distance Commuted to Work (Km)", '')::numeric,
    NULLIF("Vehicle Make", ''), NULLIF("Vehicle Model", ''), NULLIF("Vehicle Year", '')::smallint,
    NULLIF("Vehicle Type", '')
FROM stg_user;

INSERT INTO dim_product (product_id, product_name, brand, category, price_gbp, in_stock, rating)
SELECT "Product ID", "Product Name", "Brand", "Category", "Price (GBP)"::numeric, "In Stock"::boolean,
       NULLIF("Rating", '')::numeric
FROM stg_product;

INSERT INTO fact_order (order_id, customer_id, order_date, order_status, items_count, subtotal_gbp,
                        discount_code, discount_gbp, shipping_gbp, total_gbp, payment_method, platform,
                        shipping_city, country_code)
SELECT "Order ID", "Customer ID", "Order Date"::timestamptz + :'ts_shift'::interval, "Order Status", "Items Count"::integer,
       "Subtotal (GBP)"::numeric, NULLIF("Discount Code", ''), "Discount (GBP)"::numeric,
       "Shipping (GBP)"::numeric, "Total (GBP)"::numeric, "Payment Method", "Platform", "Shipping City",
       "Country Code"
FROM stg_order;

INSERT INTO fact_order_item (order_id, line_no, product_id, quantity, unit_price_gbp, line_total_gbp)
SELECT "Order ID", row_number() OVER (PARTITION BY "Order ID" ORDER BY row_no), "Product ID",
       "Quantity"::integer, "Unit Price (GBP)"::numeric, "Line Total (GBP)"::numeric
FROM stg_order_item;

INSERT INTO fact_cart_item (cart_item_id, customer_id, product_id, quantity, unit_price_gbp, added_at,
                            cart_status, converted_at, order_id)
SELECT "Cart Item ID", "Customer ID", "Product ID", "Quantity"::integer, "Unit Price (GBP)"::numeric,
       "Added At"::timestamptz + :'ts_shift'::interval, "Cart Status",
       NULLIF("Converted At", '')::timestamptz + :'ts_shift'::interval, NULLIF("Order ID", '')
FROM stg_cart_item;

INSERT INTO fact_event (event_id, customer_id, session_id, event_type, event_time, platform, product_id,
                        category, value_gbp, order_id)
SELECT "Event ID", "Customer ID", "Session ID", "Event Type", "Event Time"::timestamptz + :'ts_shift'::interval, "Platform",
       NULLIF("Product ID", ''), NULLIF("Category", ''), NULLIF("Value", '')::numeric, NULLIF("Order ID", '')
FROM stg_event;

INSERT INTO fact_consent (consent_id, customer_id, channel, consent_status, source, changed_at)
SELECT "Consent ID", "Customer ID", "Channel", "Consent Status", "Source", "Changed At"::timestamptz + :'ts_shift'::interval
FROM stg_consent;

-- ------------------------------------------------------------------ validation (abort = rollback)
DO $$
DECLARE
    t   text;
    n   bigint;
BEGIN
    FOREACH t IN ARRAY ARRAY['dim_customer', 'dim_product', 'fact_order', 'fact_order_item',
                             'fact_cart_item', 'fact_event', 'fact_consent'] LOOP
        EXECUTE format('SELECT count(*) FROM dwh.%I', t) INTO n;
        IF n = 0 THEN RAISE EXCEPTION 'seed check: dwh.% is empty', t; END IF;
        RAISE NOTICE 'seed: dwh.% = % rows', t, n;
    END LOOP;

    SELECT count(*) INTO n
    FROM dwh.dim_customer c
    JOIN LATERAL (
        SELECT bool_or(channel = 'email' AND consent_status = 'opted_in') FILTER (WHERE channel = 'email') AS email_in,
               bool_or(channel = 'sms'   AND consent_status = 'opted_in') FILTER (WHERE channel = 'sms')   AS sms_in
        FROM (SELECT DISTINCT ON (channel) channel, consent_status
              FROM dwh.fact_consent f
              WHERE f.customer_id = c.customer_id
              ORDER BY channel, changed_at DESC, consent_id DESC) last
    ) l ON true
    WHERE c.email_opt_in IS DISTINCT FROM coalesce(l.email_in, false)
       OR c.sms_opt_in   IS DISTINCT FROM coalesce(l.sms_in, false);
    IF n > 0 THEN RAISE EXCEPTION 'seed check: % customers have opt-in flags that differ from fact_consent', n; END IF;

    SELECT count(*) INTO n
    FROM dwh.fact_order o
    JOIN (SELECT order_id, sum(line_total_gbp) AS s FROM dwh.fact_order_item GROUP BY order_id) i USING (order_id)
    WHERE abs(o.subtotal_gbp - i.s) > 0.01;
    IF n > 0 THEN RAISE EXCEPTION 'seed check: % orders have subtotal <> sum of lines', n; END IF;

    SELECT count(*) INTO n
    FROM (SELECT event_time AS ts FROM dwh.fact_event
          UNION ALL SELECT order_date FROM dwh.fact_order
          UNION ALL SELECT added_at   FROM dwh.fact_cart_item
          UNION ALL SELECT changed_at FROM dwh.fact_consent) f
    WHERE ts > now();
    IF n > 0 THEN RAISE EXCEPTION 'seed check: % shifted timestamps are later than now()', n; END IF;
END $$;

COMMIT;
ANALYZE dwh.dim_customer, dwh.dim_product, dwh.fact_order, dwh.fact_order_item,
        dwh.fact_cart_item, dwh.fact_event, dwh.fact_consent;
