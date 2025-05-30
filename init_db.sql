-- creates a DB of 35 million rows
-- resulting table size is 4 GB
CREATE TABLE public.random_read_test AS
SELECT lpad(num::text,20,'0') AS PK, rpad(num::text||' - ',33,'*') AS payload
FROM (SELECT * FROM generate_series(1,35000000)) t (num);

-- create the primary index of 1 GB size.
ALTER TABLE public.random_read_test
ADD CONSTRAINT random_read_test_pk PRIMARY KEY (pk);
