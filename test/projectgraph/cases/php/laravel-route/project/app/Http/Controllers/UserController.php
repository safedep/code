<?php
namespace App\Http\Controllers;

class UserController extends Controller
{
    public function show(string $id)
    {
        return response()->json(shell_exec('id ' . $id));
    }
}
