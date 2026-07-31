<?php

namespace App\Modules\Tower\Models;

use App\Models\BaseModel;

class TowerContract extends BaseModel
{
    protected $connection = 'mongodb';
    protected $collection = 'tower_contracts';

    protected $fillable = [
        'kode',
        'tower_id',
        'vendor_id',
        'tipe_sewa',
        'biaya_bulanan',
        'tanggal_mulai',
        'tanggal_selesai',
        'status',
    ];

    protected $casts = [
        'biaya_bulanan' => 'float',
        'tanggal_mulai' => 'datetime',
        'tanggal_selesai' => 'datetime',
        'status' => 'string',
    ];

    public function tower()
    {
        return $this->belongsTo(Tower::class, 'tower_id');
    }

    public function vendor()
    {
        return $this->belongsTo(Vendor::class, 'vendor_id');
    }
}
