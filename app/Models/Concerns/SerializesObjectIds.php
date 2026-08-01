<?php

namespace App\Models\Concerns;

use MongoDB\BSON\ObjectId;

trait SerializesObjectIds
{
    /**
     * Convert semua ObjectId (termasuk _id & relasi) jadi string saat serialisasi JSON.
     * Biar konsumen API (Next.js / mobile) gak ribet sama Extended JSON.
     */
    public function toArray(): array
    {
        $array = parent::toArray();

        array_walk_recursive($array, function (&$value) {
            if ($value instanceof ObjectId) {
                $value = (string) $value;
            }
        });

        return $array;
    }
}
