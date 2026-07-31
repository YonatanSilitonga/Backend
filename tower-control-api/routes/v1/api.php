<?php

use Illuminate\Support\Facades\Route;

// Public
Route::prefix('auth')->group(base_path('routes/v1/auth.php'));
Route::get('supplychain/tracking/{noResi}', [\App\Modules\SupplyChain\Http\Controllers\TrackingController::class, 'track']);

// Protected (semua butuh token)
Route::middleware('auth:sanctum')->group(function () {
    Route::prefix('armada')->group(base_path('routes/v1/armada.php'));
    Route::prefix('supplychain')->group(base_path('routes/v1/supplychain.php'));
    Route::prefix('tower')->group(base_path('routes/v1/tower.php'));
});
