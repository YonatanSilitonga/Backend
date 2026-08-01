<?php

namespace App\Modules\SupplyChain\Models;

use App\Models\BaseModel;

class Shipment extends BaseModel
{
    protected $connection = 'mongodb';
    protected $collection = 'shipments';

    protected $fillable = [
        'no_resi',
        'pengirim',
        'penerima',
        'asal',
        'tujuan',
        'berat_kg',
        'status',
        'trip_id',
        'vehicle_id',
        'deskripsi',
    ];

    protected $casts = [
        'berat_kg' => 'float',
        'status' => 'string',
    ];
}
