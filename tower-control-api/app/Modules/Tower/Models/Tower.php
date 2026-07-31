<?php

namespace App\Modules\Tower\Models;

use App\Models\BaseModel;

class Tower extends BaseModel
{
    protected $connection = 'mongodb';
    protected $collection = 'towers';

    protected $fillable = [
        'kode',
        'nama',
        'lokasi',
        'tipe',
        'tinggi_m',
        'jumlah_tenant',
        'status',
    ];

    protected $casts = [
        'tinggi_m' => 'float',
        'jumlah_tenant' => 'integer',
        'status' => 'string',
    ];

    public function contracts()
    {
        return $this->hasMany(TowerContract::class, 'tower_id');
    }
}
