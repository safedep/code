<?php
use App\Store;
use App\DbStore;

function persist(Store $store, string $key): void
{
    $store->save($key);
}

persist(new DbStore(), $argv[1]);
