BEGIN;
DROP TABLE IF EXISTS events;
-- Leave btree_gist installed: other tables may depend on it.
COMMIT;
