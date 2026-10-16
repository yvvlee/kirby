-- Repair PostgreSQL databases initialized before seed sequences were advanced.
-- Run with the Kirby schema first in search_path. New installations do not need it.
BEGIN;

-- INSERT takes a conflicting table lock before allocating its generated ID.
-- Lock both seeded tables so the maximum IDs remain stable during the repair.
LOCK TABLE roles, permissions IN SHARE ROW EXCLUSIVE MODE;

DO $repair$
DECLARE
    table_name text;
    sequence_name regclass;
    maximum_id bigint;
    current_value bigint;
    was_called boolean;
BEGIN
    FOREACH table_name IN ARRAY ARRAY['roles', 'permissions'] LOOP
        sequence_name := pg_get_serial_sequence(table_name, 'id')::regclass;
        IF sequence_name IS NULL THEN
            RAISE EXCEPTION 'Missing ID sequence for %', table_name;
        END IF;

        EXECUTE format('SELECT MAX(id) FROM %I', table_name) INTO maximum_id;
        EXECUTE format('SELECT last_value, is_called FROM %s', sequence_name)
            INTO current_value, was_called;

        -- Preserve sequences already beyond existing IDs, including gaps from
        -- rolled-back inserts. Re-running the repair must never rewind them.
        IF maximum_id IS NOT NULL AND (
            maximum_id > current_value OR
            (maximum_id = current_value AND NOT was_called)
        ) THEN
            PERFORM setval(sequence_name, maximum_id, true);
        END IF;
    END LOOP;
END;
$repair$;

COMMIT;
