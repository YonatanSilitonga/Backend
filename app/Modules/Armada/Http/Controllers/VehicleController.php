<?php

namespace App\Modules\Armada\Http\Controllers;

use App\Http\Controllers\Controller;
use App\Modules\Armada\Models\Vehicle;
use Illuminate\Http\JsonResponse;
use Illuminate\Http\Request;

class VehicleController extends Controller
{
    public function index(Request $request): JsonResponse
    {
        $vehicles = Vehicle::query()
            ->when($request->query('status'), fn ($q, $s) => $q->where('status', $s))
            ->when($request->query('fleet_id'), fn ($q, $f) => $q->where('fleet_id', $f))
            ->get();

        return response()->json(['success' => true, 'data' => $vehicles]);
    }

    public function store(Request $request): JsonResponse
    {
        $validated = $request->validate([
            'plat' => ['required', 'string', 'max:20'],
            'tipe' => ['required', 'string', 'max:50'],
            'kapasitas_kg' => ['required', 'numeric', 'min:0'],
            'fleet_id' => ['required', 'string'],
            'status' => ['sometimes', 'string', 'in:available,in_transit,maintenance,off'],
            'tahun' => ['nullable', 'integer'],
        ]);

        $vehicle = Vehicle::create($validated);

        return response()->json(['success' => true, 'data' => $vehicle], 201);
    }

    public function show(string $id): JsonResponse
    {
        $vehicle = Vehicle::with('fleet')->find($id);

        if (! $vehicle) {
            return response()->json(['success' => false, 'message' => 'Kendaraan tidak ditemukan'], 404);
        }

        return response()->json(['success' => true, 'data' => $vehicle]);
    }

    public function update(Request $request, string $id): JsonResponse
    {
        $vehicle = Vehicle::find($id);

        if (! $vehicle) {
            return response()->json(['success' => false, 'message' => 'Kendaraan tidak ditemukan'], 404);
        }

        $validated = $request->validate([
            'plat' => ['sometimes', 'string', 'max:20'],
            'tipe' => ['sometimes', 'string', 'max:50'],
            'kapasitas_kg' => ['sometimes', 'numeric', 'min:0'],
            'fleet_id' => ['sometimes', 'string'],
            'status' => ['sometimes', 'string', 'in:available,in_transit,maintenance,off'],
            'tahun' => ['nullable', 'integer'],
        ]);

        $vehicle->update($validated);

        return response()->json(['success' => true, 'data' => $vehicle]);
    }

    public function updateStatus(Request $request, string $id): JsonResponse
    {
        $vehicle = Vehicle::find($id);

        if (! $vehicle) {
            return response()->json(['success' => false, 'message' => 'Kendaraan tidak ditemukan'], 404);
        }

        $validated = $request->validate([
            'status' => ['required', 'string', 'in:available,in_transit,maintenance,off'],
        ]);

        $vehicle->update(['status' => $validated['status']]);

        return response()->json(['success' => true, 'data' => $vehicle]);
    }

    public function destroy(string $id): JsonResponse
    {
        $vehicle = Vehicle::find($id);

        if (! $vehicle) {
            return response()->json(['success' => false, 'message' => 'Kendaraan tidak ditemukan'], 404);
        }

        $vehicle->delete();

        return response()->json(['success' => true, 'message' => 'Kendaraan dihapus']);
    }
}
