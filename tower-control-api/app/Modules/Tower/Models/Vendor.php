<?php

namespace App\Modules\Tower\Models;

use App\Models\BaseModel;

class Vendor extends BaseModel
{
    protected $connection = 'mongodb';
    protected $collection = 'vendors';

    protected $fillable = [
        'nama',
        'kontak',
        'telepon',
        'spesialisasi',
        'status',
    ];

    protected $casts = [
        'status' => 'string',
    ];
}
