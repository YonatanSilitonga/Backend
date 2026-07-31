<?php

namespace App\Modules\Tower\Models;

use App\Models\BaseModel;

class MaintenanceTask extends BaseModel
{
    protected $connection = 'mongodb';
    protected $collection = 'maintenance_tasks';

    protected $fillable = [
        'kode',
        'tower_id',
        'vendor_id',
        'jenis',
        'jadwal',
        'biaya',
        'status',
        'catatan',
    ];

    protected $casts = [
        'jadwal' => 'datetime',
        'biaya' => 'float',
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
