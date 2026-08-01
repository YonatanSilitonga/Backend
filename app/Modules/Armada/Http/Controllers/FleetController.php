<?php

namespace App\Modules\Armada\Http\Controllers;

use App\Http\Controllers\Controller;
use App\Modules\Armada\Models\Fleet;
use Illuminate\Http\JsonResponse;
use Illuminate\Http\Request;

class FleetController extends Controller
{
    public function index(Request $request): JsonResponse
    {
        $fleets = Fleet::query()
            ->when($request->query('status'), fn ($q, $s) => $q->where('status', $s))
            ->get();

        return response()->json(['success' => true, 'data' => $fleets]);
    }

    public function store(Request $request): JsonResponse
    {
        $validated = $request->validate([
            'kode' => ['required', 'string', 'max:50'],
            'nama' => ['required', 'string', 'max:255'],
            'lokasi' => ['required', 'string', 'max:255'],
            'status' => ['sometimes', 'string', 'in:active,inactive'],
            'deskripsi' => ['nullable', 'string'],
        ]);

        $fleet = Fleet::create($validated);

        return response()->json(['success' => true, 'data' => $fleet], 201);
    }

    public function show(string $id): JsonResponse
    {
        $fleet = Fleet::find($id);

        if (! $fleet) {
            return response()->json(['success' => false, 'message' => 'Fleet tidak ditemukan'], 404);
        }

        return response()->json(['success' => true, 'data' => $fleet]);
    }

    public function update(Request $request, string $id): JsonResponse
    {
        $fleet = Fleet::find($id);

        if (! $fleet) {
            return response()->json(['success' => false, 'message' => 'Fleet tidak ditemukan'], 404);
        }

        $validated = $request->validate([
            'kode' => ['sometimes', 'string', 'max:50'],
            'nama' => ['sometimes', 'string', 'max:255'],
            'lokasi' => ['sometimes', 'string', 'max:255'],
            'status' => ['sometimes', 'string', 'in:active,inactive'],
            'deskripsi' => ['nullable', 'string'],
        ]);

        $fleet->update($validated);

        return response()->json(['success' => true, 'data' => $fleet]);
    }

    public function destroy(string $id): JsonResponse
    {
        $fleet = Fleet::find($id);

        if (! $fleet) {
            return response()->json(['success' => false, 'message' => 'Fleet tidak ditemukan'], 404);
        }

        $fleet->delete();

        return response()->json(['success' => true, 'message' => 'Fleet dihapus']);
    }
}
