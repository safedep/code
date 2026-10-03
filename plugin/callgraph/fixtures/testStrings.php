<?php
function digest($data) {
    $a = hash("sha256", $data);
    $b = hash('md5', $data);
}
