-- Down migration for Module 7 channel-routing.
DROP TABLE IF EXISTS channels.send_attempts_2026_08;
DROP TABLE IF EXISTS channels.send_attempts_2026_07;
DROP TABLE IF EXISTS channels.send_attempts_2026_06;
DROP TABLE IF EXISTS channels.send_attempts_2026_05;
DROP TABLE IF EXISTS channels.send_attempts;
DROP TABLE IF EXISTS channels.supplier_channel_config;
DROP SCHEMA IF EXISTS channels;
