-- Add repository configuration and docs base dir to projects table
ALTER TABLE projects 
ADD COLUMN repositories JSONB DEFAULT '{"code_repos": [], "doc_repos": [], "reference_repos": []}',
ADD COLUMN docs_base_dir VARCHAR(500) DEFAULT '';

-- Add comments
COMMENT ON COLUMN projects.repositories IS '仓库配置 JSON（支持多个代码仓库、文档仓库、参考仓库）';
COMMENT ON COLUMN projects.docs_base_dir IS '文档输出根目录（绝对路径）';

