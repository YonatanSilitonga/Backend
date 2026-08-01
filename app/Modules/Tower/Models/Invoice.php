<?php

namespace App\Modules\Tower\Models;

use App\Models\BaseModel;

class Invoice extends BaseModel
{
    protected $connection = 'mongodb';
    protected $collection = 'invoices';

    protected $fillable = [
        'no_invoice',
        'contract_id',
        'vendor_id',
        'tower_id',
        'periode',
        'jumlah',
        'status',
        'due_date',
        'paid_at',
    ];

    protected $casts = [
        'jumlah' => 'float',
        'status' => 'string',
        'due_date' => 'datetime',
        'paid_at' => 'datetime',
    ];
}
