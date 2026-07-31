<?php

namespace App\Modules\Armada\Models;

use App\Models\BaseModel;

class Fleet extends BaseModel
{
    protected $connection = 'mongodb';
    protected $collection = 'fleets';

    protected $fillable = [
        'kode',
        'nama',
        'lokasi',
        'status',
        'deskripsi',
    ];

    protected $casts = [
        'status' => 'string',
    ];
}
