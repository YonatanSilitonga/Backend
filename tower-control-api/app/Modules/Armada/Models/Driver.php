<?php

namespace App\Modules\Armada\Models;

use App\Models\BaseModel;

class Driver extends BaseModel
{
    protected $connection = 'mongodb';
    protected $collection = 'drivers';

    protected $fillable = [
        'nama',
        'nik',
        'no_sim',
        'telepon',
        'fleet_id',
        'status',
    ];

    protected $casts = [
        'status' => 'string',
    ];

    public function fleet()
    {
        return $this->belongsTo(Fleet::class, 'fleet_id');
    }
}
