<?php

namespace App\Modules\Armada\Models;

use App\Models\BaseModel;

class Trip extends BaseModel
{
    protected $connection = 'mongodb';
    protected $collection = 'trips';

    protected $fillable = [
        'kode',
        'vehicle_id',
        'driver_id',
        'asal',
        'tujuan',
        'jarak_km',
        'status',
        'started_at',
        'completed_at',
    ];

    protected $casts = [
        'jarak_km' => 'float',
        'status' => 'string',
        'started_at' => 'datetime',
        'completed_at' => 'datetime',
    ];

    public function vehicle()
    {
        return $this->belongsTo(Vehicle::class, 'vehicle_id');
    }

    public function driver()
    {
        return $this->belongsTo(Driver::class, 'driver_id');
    }
}
