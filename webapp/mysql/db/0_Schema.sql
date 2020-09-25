DROP DATABASE IF EXISTS isuumo;
CREATE DATABASE isuumo;

DROP TABLE IF EXISTS isuumo.estate;
DROP TABLE IF EXISTS isuumo.chair;

CREATE TABLE isuumo.estate
(
    id          INTEGER             NOT NULL PRIMARY KEY,
    name        VARCHAR(64)         NOT NULL,
    description VARCHAR(4096)       NOT NULL,
    thumbnail   VARCHAR(128)        NOT NULL,
    address     VARCHAR(128)        NOT NULL,
    latitude    DOUBLE PRECISION    NOT NULL,
    longitude   DOUBLE PRECISION    NOT NULL,
    rent        INTEGER             NOT NULL,
    door_height INTEGER             NOT NULL,
    door_width  INTEGER             NOT NULL,
    features    VARCHAR(64)         NOT NULL,
    popularity  INTEGER             NOT NULL,
    door_height_id INTEGER AS ((CASE WHEN (door_height < 80) THEN 0
                                     WHEN (door_height < 110) THEN 1
                                     WHEN (door_height < 150) THEN 2
                                     ELSE 3 END)) STORED NOT NULL,
    door_width_id INTEGER AS ((CASE WHEN (door_width < 80) THEN 0
                                    WHEN (door_width < 110) THEN 1
                                    WHEN (door_width < 150) THEN 2
                                    ELSE 3 END)) STORED NOT NULL,
    rent_id INTEGER AS ((CASE WHEN (rent < 50000) THEN 0
                              WHEN (rent < 100000) THEN 1
                              WHEN (rent < 150000) THEN 2
                              ELSE 3 END)) STORED NOT NULL,
    lat_log POINT AS ((POINT(latitude, longitude))) STORED NOT NULL
    
);
CREATE INDEX rent_id_idx on isuumo.estate (rent, id);
CREATE INDEX rent_id_pupularity_id_idx ON isuumo.estate (rent_id, popularity DESC, id);
CREATE INDEX door_height_id_pupularity_id_idx ON isuumo.estate (door_height_id, popularity DESC, id);
CREATE INDEX door_width_pupularity_id_idx ON isuumo.estate (door_width_id, popularity DESC, id);

CREATE TABLE isuumo.chair
(
    id          INTEGER         NOT NULL PRIMARY KEY,
    name        VARCHAR(64)     NOT NULL,
    description VARCHAR(4096)   NOT NULL,
    thumbnail   VARCHAR(128)    NOT NULL,
    price       INTEGER         NOT NULL,
    height      INTEGER         NOT NULL,
    width       INTEGER         NOT NULL,
    depth       INTEGER         NOT NULL,
    color       VARCHAR(64)     NOT NULL,
    features    VARCHAR(64)     NOT NULL,
    kind        VARCHAR(64)     NOT NULL,
    popularity  INTEGER         NOT NULL,
    stock       INTEGER         NOT NULL,
    height_id INTEGER AS ((CASE WHEN (height < 80) THEN 0
                                WHEN (height < 110) THEN 1
                                WHEN (height < 150) THEN 2
                                ELSE 3 END)) STORED NOT NULL,
    width_id INTEGER AS ((CASE WHEN (width < 80) THEN 0
                               WHEN (width < 110) THEN 1
                               WHEN (width < 150) THEN 2
                               ELSE 3 END)) STORED NOT NULL,
    depth_id INTEGER AS ((CASE WHEN (depth < 80) THEN 0
                               WHEN (depth < 110) THEN 1
                               WHEN (depth < 150) THEN 2
                               ELSE 3 END)) STORED NOT NULL,
    price_id INTEGER AS ((CASE WHEN (price < 3000) THEN 0
                               WHEN (price < 6000) THEN 1
                               WHEN (price < 9000) THEN 2
                               WHEN (price < 12000) THEN 3
                               WHEN (price < 15000) THEN 4
                               ELSE 5 END)) STORED NOT NULL
);
CREATE INDEX price_id_idx on isuumo.chair (price, id);
CREATE INDEX stock_price_id_idx on isuumo.chair (stock, price, id);

SET GLOBAL slow_query_log='ON';
SET GLOBAL long_query_time=0;
SET GLOBAL slow_query_log_file='/var/log/mysql/slow.log';

