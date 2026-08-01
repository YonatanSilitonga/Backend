<?php

namespace App\Modules\Tower\Http\Controllers;

use App\Http\Controllers\Controller;
use App\Modules\Tower\Models\Tower;
use Illuminate\Http\JsonResponse;
use Illuminate\Http\Request;

class TowerController extends Controller
{
    public function index(Request $request): JsonResponse
    {
        $towers = Tower::query()
            ->when($request->query('status'), fn ($q, $s) => $q->where('status', $s))
            ->when($request->query('lokasi'), fn ($q, $l) => $q->where('lokasi', $l))
            ->get();

        return response()->json(['success' => true, 'data' => $towers]);
    }

    public function store(Request $request): JsonResponse
    {
        $validated = $request->validate([
            'kode' => ['required', 'string', 'max:50'],
            'nama' => ['required', 'string', 'max:255'],
            'lokasi' => ['required', 'string', 'max:255'],
            'tipe' => ['required', 'string', 'max:100'],
            'tinggi_m' => ['nullable', 'numeric', 'min:0'],
            'jumlah_tenant' => ['nullable', 'integer', 'min:0'],
            'status' => ['sometimes', 'string', 'in:active,inactive,maintenance'],
        ]);

        $tower = Tower::create($validated);

        return response()->json(['success' => true, 'data' => $tower], 201);
    }

    public function show(string $id): JsonResponse
    {
        $tower = Tower::with('contracts')->find($id);

        if (! $tower) {
            return response()->json(['success' => false, 'message' => 'Tower tidak ditemukan'], 404);
        }

        return response()->json(['success' => true, 'data' => $tower]);
    }

    public function update(Request $request, string $id): JsonResponse
    {
        $tower = Tower::find($id);

        if (! $tower) {
            return response()->json(['success' => false, 'message' => 'Tower tidak ditemukan'], 404);
        }

        $validated = $request->validate([
            'kode' => ['sometimes', 'string', 'max:50'],
            'nama' => ['sometimes', 'string', 'max:255'],
            'lokasi' => ['sometimes', 'string', 'max:255'],
            'tipe' => ['sometimes', 'string', 'max:100'],
            'tinggi_m' => ['nullable', 'numeric', 'min:0'],
            'jumlah_tenant' => ['nullable', 'integer', 'min:0'],
            'status' => ['sometimes', 'string', 'in:active,inactive,maintenance'],
        ]);

        $tower->update($validated);

        return response()->json(['success' => true, 'data' => $tower]);
    }

    public function destroy(string $id): JsonResponse
    {
        $tower = Tower::find($id);

        if (! $tower) {
            return response()->json(['success' => false, 'message' => 'Tower tidak ditemukan'], 404);
        }

        $tower->delete();

        return response()->json(['success' => true, 'message' => 'Tower dihapus']);
    }
}
