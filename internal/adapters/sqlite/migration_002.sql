BEGIN IMMEDIATE;
CREATE TABLE project_config(key TEXT PRIMARY KEY NOT NULL, value TEXT NOT NULL);
INSERT INTO project_config(key,value) VALUES ('trash_retention_days','30');
PRAGMA user_version=2;
COMMIT;
