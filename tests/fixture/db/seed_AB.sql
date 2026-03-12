INSERT INTO activities (id, title, channel_name, page_url, type) VALUES ('ACT-A', 'Title A', 'LINE 購物', 'https://buy.line.me/A', 'unknown');
INSERT INTO sync_state (key, hash, synced_at) VALUES ('activity_list', 'efb18485cd0b8c04bcfe34e6255146b96e625a74ff4ba99cd7feaff87af2f0ca', '2026-03-01 00:00:00');
INSERT INTO sync_state (key, hash, synced_at) VALUES ('activity:ACT-A', '47f7637841ee4dc478cd84a32f654b4f5351a9a40518cf08047970d65b7410f4', '2026-03-01 00:00:00');

INSERT INTO activities (id, title, channel_name, page_url, type) VALUES ('ACT-B', 'Title B', 'LINE 購物', 'https://buy.line.me/B', 'unknown');
INSERT INTO sync_state (key, hash, synced_at) VALUES ('activity:ACT-B', 'af5ebd868df5ed1a28a3068e578c78bf5ce93660fbaf355cf3c07804473491ea', '2026-03-01 00:00:00');
