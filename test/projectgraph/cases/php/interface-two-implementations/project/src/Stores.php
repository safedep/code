<?php
namespace App;

interface Store
{
    public function save(string $key): void;
}

class DbStore implements Store
{
    public function save(string $key): void
    {
        mysqli_query(null, $key);
    }
}

class FileStore implements Store
{
    public function save(string $key): void
    {
        unlink($key);
    }
}
