INSERT INTO activities (id, title, channel_name, page_url, type, valid_from, valid_until, is_active, created_at, updated_at) VALUES
('ACT-A', 'LINE 購物集點', 'LINE 購物', 'http://a', 'keyword', 1772294400, 1774972799, 1, '2026-03-01 00:00:00', '2026-03-01 00:00:00');

INSERT INTO daily_tasks (activity_id, use_date, keyword, url, note) VALUES
('ACT-A', '2026-03-05', 'SHOP0305', '', '');
