<?php

namespace App\Sanctum;

class NewAccessToken extends \Laravel\Sanctum\NewAccessToken
{
    /**
     * Override konstruktor biar terima model token custom (MongoDB).
     */
    public function __construct($accessToken, string $plainTextToken)
    {
        $this->accessToken = $accessToken;
        $this->plainTextToken = $plainTextToken;
    }
}
