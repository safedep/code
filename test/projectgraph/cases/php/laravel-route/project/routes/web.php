<?php
use App\Http\Controllers\UserController;
use App\Support\Audit;
use Illuminate\Support\Facades\Route;

Route::get('/users/{id}', [UserController::class, 'show']);
Route::get('/audit', [Audit::class, 'record']);
