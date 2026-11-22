-- CQL skripta za rucno kreiranje i popunjavanje Cassandra baze podacima
-- Pokretanje u Dockeru:
-- docker exec -i gdrive-cassandra cqlsh < services/notifications-service/scripts/seed.cql

CREATE KEYSPACE IF NOT EXISTS notifications_ks
WITH replication = {'class': 'SimpleStrategy', 'replication_factor': 1};

USE notifications_ks;

CREATE TABLE IF NOT EXISTS notifications_by_user (
    user_id text,
    created_at timestamp,
    id text,
    type text,
    resource_id text,
    resource_name text,
    message text,
    is_read boolean,
    PRIMARY KEY ((user_id), created_at, id)
) WITH CLUSTERING ORDER BY (created_at DESC, id ASC);

-- Primer rucnog unosa notifikacija (zameniti <USER_ID> ID-jem korisnika iz MongoDB kolekcije 'users'):
INSERT INTO notifications_by_user (user_id, created_at, id, type, resource_id, resource_name, message, is_read)
VALUES ('<USER_ID>', toTimestamp(now()), 'manual-notif-1', 'FILE_SHARED', 'file-101', 'Izvestaj_Bezbednost.pdf', 'Korisnik  je podelio datoteku Izvestaj_Bezbednost.pdf sa vama.', false);
