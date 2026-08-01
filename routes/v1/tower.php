<?php

use App\Modules\Tower\Http\Controllers\ContractController;
use App\Modules\Tower\Http\Controllers\InvoiceController;
use App\Modules\Tower\Http\Controllers\MaintenanceController;
use App\Modules\Tower\Http\Controllers\TowerController;
use App\Modules\Tower\Http\Controllers\VendorController;
use Illuminate\Support\Facades\Route;

Route::apiResource('towers', TowerController::class);

Route::apiResource('vendors', VendorController::class);

Route::apiResource('contracts', ContractController::class);

Route::apiResource('maintenance', MaintenanceController::class);

Route::apiResource('invoices', InvoiceController::class)->except(['store', 'update', 'destroy']);
Route::post('invoices/generate', [InvoiceController::class, 'generate']);
Route::patch('invoices/{id}/paid', [InvoiceController::class, 'markPaid']);
Route::get('invoices/billing/summary', [InvoiceController::class, 'summary']);
