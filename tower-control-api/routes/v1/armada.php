<?php

use App\Modules\Armada\Http\Controllers\DriverController;
use App\Modules\Armada\Http\Controllers\FleetController;
use App\Modules\Armada\Http\Controllers\TripController;
use App\Modules\Armada\Http\Controllers\VehicleController;
use Illuminate\Support\Facades\Route;

Route::apiResource('fleets', FleetController::class);
Route::patch('fleets/{id}/status', [FleetController::class, 'updateStatus']);

Route::apiResource('vehicles', VehicleController::class);
Route::patch('vehicles/{id}/status', [VehicleController::class, 'updateStatus']);

Route::apiResource('drivers', DriverController::class);
Route::patch('drivers/{id}/status', [DriverController::class, 'updateStatus']);

Route::apiResource('trips', TripController::class);
Route::patch('trips/{id}/status', [TripController::class, 'updateStatus']);
