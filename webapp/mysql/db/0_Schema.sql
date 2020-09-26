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
    lat_log POINT AS ((POINT(latitude, longitude))) STORED SRID 0 NOT NULL,
    features_set SET(
      "最上階",
      "防犯カメラ",
      "ウォークインクローゼット",
      "ワンルーム",
      "ルーフバルコニー付",
      "エアコン付き",
      "駐輪場あり",
      "プロパンガス",
      "駐車場あり",
      "防音室",
      "追い焚き風呂",
      "オートロック",
      "即入居可",
      "IHコンロ",
      "敷地内駐車場",
      "トランクルーム",
      "角部屋",
      "カスタマイズ可",
      "DIY可",
      "ロフト",
      "シューズボックス",
      "インターネット無料",
      "地下室",
      "敷地内ゴミ置場",
      "管理人有り",
      "宅配ボックス",
      "ルームシェア可",
      "セキュリティ会社加入済",
      "メゾネット",
      "女性限定",
      "バイク置場あり",
      "エレベーター",
      "ペット相談可",
      "洗面所独立",
      "都市ガス",
      "浴室乾燥機",
      "インターネット接続可",
      "テレビ・通信",
      "専用庭",
      "システムキッチン",
      "高齢者歓迎",
      "ケーブルテレビ",
      "床下収納",
      "バス・トイレ別",
      "駐車場2台以上",
      "楽器相談可",
      "フローリング",
      "オール電化",
      "TVモニタ付きインタホン",
      "デザイナーズ物件"
    ) AS (features) STORED NOT NULL
);
CREATE INDEX rentid_idx on isuumo.estate (rent_id);
CREATE INDEX rent_id_idx on isuumo.estate (rent, id);
CREATE INDEX rentid_pupularity_id_idx ON isuumo.estate (rent_id, popularity DESC, id);
CREATE INDEX doorheightid_pupularity_id_idx ON isuumo.estate (door_height_id, popularity DESC, id);
CREATE INDEX doorwidthid_pupularity_id_idx ON isuumo.estate (door_width_id, popularity DESC, id);
CREATE INDEX doorwidthid_rentid_idx ON isuumo.estate (door_width_id, rent_id);
CREATE INDEX doorheightid_rentid_idx ON isuumo.estate (door_height_id, rent_id);
CREATE INDEX lat_log_idx ON isuumo.estate (lat_log);
CREATE INDEX popularity_id_idx ON isuumo.estate (popularity desc, id);

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
    color       ENUM(
      "黒",
      "白",
      "赤",
      "青",
      "緑",
      "黄",
      "紫",
      "ピンク",
      "オレンジ",
      "水色",
      "ネイビー",
      "ベージュ"
    )     NOT NULL,
    features    VARCHAR(64)     NOT NULL,
    kind        ENUM(
      "ゲーミングチェア",
      "座椅子",
      "エルゴノミクス",
      "ハンモック"
    )     NOT NULL,
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
                               ELSE 5 END)) STORED NOT NULL,
    features_set SET(
      "ヘッドレスト付き",
      "肘掛け付き",
      "キャスター付き",
      "アーム高さ調節可能",
      "リクライニング可能",
      "高さ調節可能",
      "通気性抜群",
      "メタルフレーム",
      "低反発",
      "木製",
      "背もたれつき",
      "回転可能",
      "レザー製",
      "昇降式",
      "デザイナーズ",
      "金属製",
      "プラスチック製",
      "法事用",
      "和風",
      "中華風",
      "西洋風",
      "イタリア製",
      "国産",
      "背もたれなし",
      "ラテン風",
      "布貼地",
      "スチール製",
      "メッシュ貼地",
      "オフィス用",
      "料理店用",
      "自宅用",
      "キャンプ用",
      "クッション性抜群",
      "モーター付き",
      "ベッド一体型",
      "ディスプレイ配置可能",
      "ミニ机付き",
      "スピーカー付属",
      "中国製",
      "アンティーク",
      "折りたたみ可能",
      "重さ500g以内",
      "24回払い無金利",
      "現代的デザイン",
      "近代的なデザイン",
      "ルネサンス的なデザイン",
      "アームなし",
      "オーダーメイド可能",
      "ポリカーボネート製",
      "フットレスト付き"
    ) AS (features) STORED NOT NULL
);
CREATE INDEX price_id_idx on isuumo.chair (price, id);
CREATE INDEX stock_price_id_idx on isuumo.chair (stock, price, id);
CREATE INDEX popularity_id_idx on isuumo.chair (popularity desc, id);

SET GLOBAL slow_query_log='ON';
SET GLOBAL long_query_time=0;
SET GLOBAL slow_query_log_file='/var/log/mysql/slow.log';

