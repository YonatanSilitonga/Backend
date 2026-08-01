<?php

use App\Modules\SupplyChain\Http\Controllers\ControlTowerController;
use App\Modules\SupplyChain\Http\Controllers\ShipmentController;
use App\Modules\SupplyChain\Http\Controllers\TrackingController;
use Illuminate\Support\Facades\Route;

Route::get('control-tower/summary', [ControlTowerController::class, 'summary'])
    ->middleware('role:admin,supervisor');

Route::apiResource('shipments', ShipmentController::class);
Route::patch('shipments/{id}/status', [ShipmentController::class, 'updateStatus']);

Route::get('shipments/{id}/tracking', [TrackingController::class, 'history']);
Route::post('shipments/{id}/tracking', [TrackingController::class, 'store']);
