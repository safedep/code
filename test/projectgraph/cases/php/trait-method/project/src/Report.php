<?php
namespace App;

class Report
{
    use Uploads;

    public function publish(string $name): void
    {
        $this->upload($name);
    }
}
