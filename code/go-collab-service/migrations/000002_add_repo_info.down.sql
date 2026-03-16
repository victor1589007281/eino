-- Remove repository configuration and docs base dir columns from projects table
ALTER TABLE projects 
DROP COLUMN repositories,
DROP COLUMN docs_base_dir;
