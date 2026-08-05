-- Reverse 0037: collapse duplicate-role rows to (video_id, person_id) and drop the
-- role/orphan columns. A person linked in two roles on one video collapses to one row.
CREATE TABLE video_people_old (
    video_id  INTEGER NOT NULL REFERENCES videos(id) ON DELETE CASCADE,
    person_id INTEGER NOT NULL REFERENCES people(id) ON DELETE CASCADE,
    PRIMARY KEY (video_id, person_id)
);
INSERT OR IGNORE INTO video_people_old (video_id, person_id)
SELECT DISTINCT video_id, person_id FROM video_people;
DROP TABLE video_people;
ALTER TABLE video_people_old RENAME TO video_people;
CREATE INDEX idx_video_people_person ON video_people(person_id);

ALTER TABLE people DROP COLUMN orphaned_at;
