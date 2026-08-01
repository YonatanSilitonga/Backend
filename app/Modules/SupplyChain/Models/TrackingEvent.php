<?php

namespace App\Modules\SupplyChain\Models;

use App\Models\BaseModel;

class TrackingEvent extends BaseModel
{
    protected $connection = 'mongodb';
    protected $collection = 'tracking_events';

    protected $fillable = [
        'shipment_id',
        'status',
        'lokasi',
        'latitude',
        'longitude',
        'deskripsi',
        'event_time',
    ];

    protected $casts = [
        'latitude' => 'float',
        'longitude' => 'float',
        'status' => 'string',
        'event_time' => 'datetime',
    ];
}
