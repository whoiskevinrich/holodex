-- Reverse 0019: drop the studio-logo cache index. On-disk bytes under
-- DATA_PATH/studio-logos/ are left for the operator to remove (golang-migrate does
-- not touch the filesystem), matching the person-image down migration's posture.
DROP TABLE IF EXISTS studio_logos;
