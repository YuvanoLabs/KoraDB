#!/usr/bin/env bash
# End-to-end KoraDB demo. Builds the CLI and walks through schema -> collection
# -> insert -> query -> schema evolution against a fresh database file.
set -euo pipefail
cd "$(dirname "$0")/.."

go build -o bin/KoraDB ./cmd/KoraDB
db="$(mktemp -u)-KoraDB-demo.db"
p="./bin/KoraDB"

echo; echo "== 1. register schema =="
"$p" --db "$db" schema add user.proto ./examples/user.proto

echo; echo "== 2. create collection (key=email, index=city) =="
"$p" --db "$db" collection create users example.User --key-field=email --index=city

echo; echo "== 3. insert =="
"$p" --db "$db" insert users '{"name":"Alice","email":"alice@x.com","city":"NYC"}'
"$p" --db "$db" insert users '{"name":"Bob","email":"bob@x.com","city":"LA"}'
"$p" --db "$db" insert users '{"name":"Carol","email":"carol@x.com","city":"NYC"}'

echo; echo "== 4. query indexed field city==NYC =="
"$p" --db "$db" query users city == NYC

echo; echo "== 5. evolve schema (add age, roles) — no migration =="
"$p" --db "$db" schema add user.proto ./examples/user_v2.proto
echo "-- old record still reads cleanly --"
"$p" --db "$db" get users alice@x.com
echo "-- new record uses new fields --"
"$p" --db "$db" insert users '{"name":"Dave","email":"dave@x.com","city":"SF","age":42,"roles":["admin","ops"]}'
"$p" --db "$db" get users dave@x.com

echo; echo "Demo DB: $db"
