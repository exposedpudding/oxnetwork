-- Verified service health (ADR-014): record which endpoint a probe checked and a
-- short result alongside each service observation.
ALTER TABLE service_observations ADD COLUMN endpoint TEXT NOT NULL DEFAULT '';
ALTER TABLE service_observations ADD COLUMN detail TEXT NOT NULL DEFAULT '';
