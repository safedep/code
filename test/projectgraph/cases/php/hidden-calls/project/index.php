<?php
use App\Job;

$job = new Job();
$job->run('a');
echo $job;
call_user_func('App\Job::tick');
array_map('App\trim_all', ['x']);
\App\Util\notify('done');
Job::create();
