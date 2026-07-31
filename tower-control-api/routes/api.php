<?php

use Illuminate\Support\Facades\Route;

// Safety net: redirect route buat guest (biar Sanctum gak crash di API-only mode)
Route::any('login', function () {
    return response()->json([
        'success' => false,
        'message' => 'Unauthenticated. Silakan login terlebih dahulu.',
    ], 401);
})->name('login');

Route::prefix('v1')->group(base_path('routes/v1/api.php'));
