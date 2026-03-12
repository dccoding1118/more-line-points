INSERT INTO activities (id, title, channel_name, page_url, type, action_url, valid_from, valid_until, is_active) VALUES
('A1', 'KW任務', 'LINE 購物', 'http://a1', 'keyword', '', '2026-03-01', '2026-03-31', 1),
('A2', 'Shop任務', '', 'http://a2', 'shop-collect', 'http://shop', '2026-03-01', '2026-03-31', 1),
('A3', 'Lucky任務', '', 'http://a3', 'lucky-draw', 'http://lucky', '2026-03-01', '2026-03-31', 1),
('A4', 'Poll任務', '', 'http://a4', 'poll', 'http://poll', '2026-03-01', '2026-03-31', 1),
('A5', 'App任務', '', 'http://a5', 'app-checkin', 'http://app', '2026-03-01', '2026-03-31', 1),
('A6', 'Passport任務', '', 'http://a6', 'passport', 'http://passport', '2026-03-01', '2026-03-31', 1),
('A7', 'Share任務', '', 'http://a7', 'share', 'http://share', '2026-03-01', '2026-03-31', 1),
('A8', 'Other任務', '', 'http://a8', 'other', 'http://other', '2026-03-01', '2026-03-31', 1);

INSERT INTO daily_tasks (activity_id, use_date, keyword, url, note) VALUES
('A1', '2026-03-05', 'K1', '', ''),
('A2', '2026-03-05', '', '', '');
