<?php

namespace App\Models;

use App\Models\Concerns\SerializesObjectIds;
use Illuminate\Database\Eloquent\Factories\HasFactory;
use Laravel\Sanctum\Contracts\HasAbilities;
use MongoDB\Laravel\Eloquent\Model;

class PersonalAccessToken extends Model implements HasAbilities
{
    use HasFactory, SerializesObjectIds;

    protected $connection = 'mongodb';
    protected $collection = 'personal_access_tokens';

    protected $fillable = [
        'name',
        'token',
        'abilities',
        'expires_at',
        'last_used_at',
    ];

    protected $hidden = [
        'token',
    ];

    protected $casts = [
        'last_used_at' => 'datetime',
        'expires_at' => 'datetime',
        'abilities' => 'json',
    ];

    public function tokenable()
    {
        return $this->morphTo();
    }

    public function can($ability): bool
    {
        $abilities = $this->abilities ?? ['*'];

        return in_array('*', $abilities, true) || in_array($ability, $abilities, true);
    }

    public function cant($ability): bool
    {
        return ! $this->can($ability);
    }

    /**
     * Cari token dari string plain text (dipakai Sanctum Guard).
     */
    public static function findToken(string $token): ?self
    {
        if (strpos($token, '|') === false) {
            return static::where('token', hash('sha256', $token))->first();
        }

        [$id, $token] = explode('|', $token, 2);

        $instance = static::find($id);

        if (! $instance || ! hash_equals($instance->token, hash('sha256', $token))) {
            return null;
        }

        return $instance;
    }
}
