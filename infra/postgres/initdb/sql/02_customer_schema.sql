-- customer_db / dwh: filtered warehouse copy of the customer stub (star schema).
-- Objects are created as customer_svc so the service role owns them.
CREATE SCHEMA dwh AUTHORIZATION customer_svc;
SET ROLE customer_svc;
SET search_path = dwh;

CREATE TABLE dim_customer (
    customer_id             text PRIMARY KEY,
    first_name              text NOT NULL,
    last_name               text NOT NULL,
    sex                     text NOT NULL CHECK (sex IN ('Male', 'Female')),
    date_of_birth           date NOT NULL,
    age_years               smallint NOT NULL CHECK (age_years BETWEEN 0 AND 130),
    marital_status          text NOT NULL,
    dependants              smallint CHECK (dependants >= 0),
    email                   text NOT NULL UNIQUE,
    email_bounced           boolean NOT NULL,
    phone_e164              text CHECK (phone_e164 ~ '^\+[1-9][0-9]{6,14}$'),
    country_code            char(2) NOT NULL CHECK (country_code IN ('TR', 'GB')),
    city                    text NOT NULL,
    language                text NOT NULL CHECK (language IN ('tr', 'en')),
    timezone                text NOT NULL,
    address_street          text,
    address_city            text,
    address_postcode        text,
    signup_date             date NOT NULL,
    account_status          text NOT NULL CHECK (account_status IN ('active', 'inactive', 'deleted')),
    deleted_at              timestamptz,
    last_active_date        date NOT NULL,
    preferred_platform      text NOT NULL CHECK (preferred_platform IN ('ios', 'android', 'web')),
    email_opt_in            boolean NOT NULL,
    sms_opt_in              boolean NOT NULL,
    consent_updated_at      timestamptz NOT NULL,
    total_orders            integer NOT NULL CHECK (total_orders >= 0),
    total_spend_gbp         numeric(12,2) NOT NULL,
    avg_order_value_gbp     numeric(12,2) NOT NULL,
    first_order_date        date,
    last_order_date         date,
    favourite_category      text,
    last_cart_add_date      date,
    has_open_cart           boolean NOT NULL,
    open_cart_value_gbp     numeric(12,2) NOT NULL,
    sessions_90d            integer NOT NULL,
    loyalty_tier            text NOT NULL CHECK (loyalty_tier IN ('Bronze', 'Silver', 'Gold', 'Platinum')),
    loyalty_points          integer NOT NULL,
    campaigns_received_90d  integer NOT NULL,
    emails_opened_90d       integer NOT NULL,
    last_campaign_sent_at   timestamptz,
    employer_company        text,
    yearly_salary_gbp       integer,
    yearly_pension_gbp      integer,
    retired                 boolean NOT NULL,
    commute_distance_km     numeric(5,2),
    vehicle_make            text,
    vehicle_model           text,
    vehicle_year            smallint,
    vehicle_type            text,
    CHECK ((account_status = 'deleted') = (deleted_at IS NOT NULL))
);
COMMENT ON TABLE dim_customer IS 'One wide row per customer. Card number/dates, CVV and IBAN from the raw stub are deliberately excluded.';
COMMENT ON COLUMN dim_customer.city IS 'Segmentation city. address_* columns are the raw UK address and must not be used for city filters.';

CREATE TABLE dim_product (
    product_id    text PRIMARY KEY,
    product_name  text NOT NULL,
    brand         text NOT NULL,
    category      text NOT NULL,
    price_gbp     numeric(12,2) NOT NULL CHECK (price_gbp >= 0),
    in_stock      boolean NOT NULL,
    rating        numeric(2,1) CHECK (rating BETWEEN 0 AND 5)
);

CREATE TABLE fact_order (
    order_id        text PRIMARY KEY,
    customer_id     text NOT NULL REFERENCES dim_customer,
    order_date      timestamptz NOT NULL,
    order_status    text NOT NULL CHECK (order_status IN ('shipped', 'delivered', 'cancelled', 'returned', 'refunded')),
    items_count     integer NOT NULL CHECK (items_count > 0),
    subtotal_gbp    numeric(12,2) NOT NULL,
    discount_code   text,
    discount_gbp    numeric(12,2) NOT NULL,
    shipping_gbp    numeric(12,2) NOT NULL,
    total_gbp       numeric(12,2) NOT NULL,
    payment_method  text NOT NULL,
    platform        text NOT NULL,
    shipping_city   text NOT NULL,
    country_code    char(2) NOT NULL
);

CREATE TABLE fact_order_item (
    order_id        text NOT NULL REFERENCES fact_order,
    line_no         smallint NOT NULL,
    product_id      text NOT NULL REFERENCES dim_product,
    quantity        integer NOT NULL CHECK (quantity > 0),
    unit_price_gbp  numeric(12,2) NOT NULL,
    line_total_gbp  numeric(12,2) NOT NULL,
    PRIMARY KEY (order_id, line_no)   -- the same product can appear twice in one order
);

CREATE TABLE fact_cart_item (
    cart_item_id    text PRIMARY KEY,
    customer_id     text NOT NULL REFERENCES dim_customer,
    product_id      text NOT NULL REFERENCES dim_product,
    quantity        integer NOT NULL CHECK (quantity > 0),
    unit_price_gbp  numeric(12,2) NOT NULL,
    added_at        timestamptz NOT NULL,
    cart_status     text NOT NULL CHECK (cart_status IN ('open', 'abandoned', 'converted', 'removed')),
    converted_at    timestamptz,
    order_id        text REFERENCES fact_order,
    CHECK ((cart_status = 'converted') = (order_id IS NOT NULL))
);

CREATE TABLE fact_event (
    event_id     text PRIMARY KEY,
    customer_id  text NOT NULL REFERENCES dim_customer,
    session_id   text NOT NULL,
    event_type   text NOT NULL CHECK (event_type IN ('app_open', 'page_view', 'search', 'product_view', 'wishlist_add',
                                                     'add_to_cart', 'remove_from_cart', 'checkout_started', 'purchase')),
    event_time   timestamptz NOT NULL,
    platform     text NOT NULL,
    product_id   text REFERENCES dim_product,
    category     text,
    value_gbp    numeric(12,2),
    order_id     text REFERENCES fact_order
);

CREATE TABLE fact_consent (
    consent_id      text PRIMARY KEY,
    customer_id     text NOT NULL REFERENCES dim_customer,
    channel         text NOT NULL CHECK (channel IN ('email', 'sms')),
    consent_status  text NOT NULL CHECK (consent_status IN ('opted_in', 'opted_out')),
    source          text NOT NULL,
    changed_at      timestamptz NOT NULL
);

-- Segmentation access paths
CREATE INDEX dim_customer_city_idx            ON dim_customer (city);
CREATE INDEX dim_customer_country_idx         ON dim_customer (country_code);
CREATE INDEX dim_customer_last_cart_add_idx   ON dim_customer (last_cart_add_date);
CREATE INDEX dim_customer_last_order_idx      ON dim_customer (last_order_date);
CREATE INDEX fact_order_customer_date_idx     ON fact_order (customer_id, order_date);
CREATE INDEX fact_order_item_product_idx      ON fact_order_item (product_id);
CREATE INDEX fact_cart_item_customer_idx      ON fact_cart_item (customer_id, added_at);
CREATE INDEX fact_cart_item_product_idx       ON fact_cart_item (product_id);
CREATE INDEX fact_cart_item_order_idx         ON fact_cart_item (order_id);
CREATE INDEX fact_event_customer_time_idx     ON fact_event (customer_id, event_time);
CREATE INDEX fact_event_type_time_idx         ON fact_event (event_type, event_time);
CREATE INDEX fact_event_product_idx           ON fact_event (product_id);
CREATE INDEX fact_event_order_idx             ON fact_event (order_id);
CREATE INDEX fact_consent_customer_idx        ON fact_consent (customer_id, channel, changed_at);

RESET ROLE;
