ALTER TABLE test_results DROP CONSTRAINT test_results_site_id_fkey;
ALTER TABLE test_results ADD CONSTRAINT test_results_site_id_fkey
    FOREIGN KEY (site_id) REFERENCES sites(id) ON DELETE CASCADE;
