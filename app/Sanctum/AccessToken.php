<?php

namespace App\Sanctum;

use Illuminate\Contracts\Support\Arrayable;
use Illuminate\Contracts\Support\Jsonable;
use JsonSerializable;

/**
 * Hasil createToken yang kompatibel dengan model token MongoDB.
 * Menggantikan Laravel\Sanctum\NewAccessToken yang type-hint-nya ketat ke model SQL.
 */
class AccessToken implements Arrayable, Jsonable, JsonSerializable
{
    public function __construct(public mixed $accessToken, public string $plainTextToken)
    {
    }

    public function toArray(): array
    {
        return [
            'accessToken' => $this->accessToken,
            'plainTextToken' => $this->plainTextToken,
        ];
    }

    public function toJson($options = 0): string
    {
        return json_encode($this->toArray(), $options);
    }

    public function jsonSerialize(): mixed
    {
        return $this->toArray();
    }
}
