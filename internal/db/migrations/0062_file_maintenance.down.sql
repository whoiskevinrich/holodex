-- Reverse 0062. Maintenance rows have no meaning to a pre-F77 worker (it would hand their
-- payload to the tag writer), so they are deleted before the column goes. Held originals on
-- disk are not touched: the files survive as `.holodex-orig` siblings the scanner ignores.
DROP TABLE IF EXISTS held_originals;
DELETE FROM writeback_queue WHERE kind = 'maintenance';
DROP INDEX IF EXISTS idx_writeback_queue_kind_status;
ALTER TABLE writeback_queue DROP COLUMN kind;
