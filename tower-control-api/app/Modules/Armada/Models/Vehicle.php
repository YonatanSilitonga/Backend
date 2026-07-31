<?php

namespace App\Modules\Armada\Models;

use App\Models\BaseModel;

class Vehicle extends BaseModel
{
    protected $connection = 'mongodb';
    protected $collection = 'vehicles';

    protected $fillable = [
        'plat',
        'tipe',
        'kapasitas_kg',
        'fleet_id',
        'status',
        'tahun',
    ];

    protected $casts = [
        'kapasitas_kg' => 'integer',
        'tahun' => 'integer',
        'status' => 'string',
    ];

    public function fleet()
    {
        return $this->belongsTo(Fleet::class, 'fleet_id');
    }
}
