-- Reverse 0021: drop the provider-icon cache index. On-disk bytes under
-- DATA_PATH/provider-icons/ are left for the operator to remove (golang-migrate does
-- not touch the filesystem), matching the studio-logo / person-image down posture.
DROP TABLE IF EXISTS provider_icons;
