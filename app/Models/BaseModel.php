<?php

namespace App\Models;

use App\Models\Concerns\SerializesObjectIds;
use MongoDB\Laravel\Eloquent\Model;

abstract class BaseModel extends Model
{
    use SerializesObjectIds;
}
