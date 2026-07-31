<?php

namespace App\Modules\Armada\Http\Controllers;

use App\Http\Controllers\Controller;
use App\Modules\Armada\Models\Driver;
use Illuminate\Http\JsonResponse;
use Illuminate\Http\Request;

class DriverController extends Controller
{
    public function index(Request $request): JsonResponse
    {
        $drivers = Driver::query()
            ->when($request->query('status'), fn ($q, $s) => $q->where('status', $s))
            ->when($request->query('fleet_id'), fn ($q, $f) => $q->where('fleet_id', $f))
            ->get();

        return response()->json(['success' => true, 'data' => $drivers]);
    }

    public function store(Request $request): JsonResponse
    {
        $validated = $request->validate([
            'nama' => ['required', 'string', 'max:255'],
            'nik' => ['required', 'string', 'max:30'],
            'no_sim' => ['required', 'string', 'max:30'],
            'telepon' => ['required', 'string', 'max:20'],
            'fleet_id' => ['required', 'string'],
            'status' => ['sometimes', 'string', 'in:on_duty,off'],
        ]);

        $driver = Driver::create($validated);

        return response()->json(['success' => true, 'data' => $driver], 201);
    }

    public function show(string $id): JsonResponse
    {
        $driver = Driver::with('fleet')->find($id);

        if (! $driver) {
            return response()->json(['success' => false, 'message' => 'Driver tidak ditemukan'], 404);
        }

        return response()->json(['success' => true, 'data' => $driver]);
    }

    public function update(Request $request, string $id): JsonResponse
    {
        $driver = Driver::find($id);

        if (! $driver) {
            return response()->json(['success' => false, 'message' => 'Driver tidak ditemukan'], 404);
        }

        $validated = $request->validate([
            'nama' => ['sometimes', 'string', 'max:255'],
            'nik' => ['sometimes', 'string', 'max:30'],
            'no_sim' => ['sometimes', 'string', 'max:30'],
            'telepon' => ['sometimes', 'string', 'max:20'],
            'fleet_id' => ['sometimes', 'string'],
            'status' => ['sometimes', 'string', 'in:on_duty,off'],
        ]);

        $driver->update($validated);

        return response()->json(['success' => true, 'data' => $driver]);
    }

    public function updateStatus(Request $request, string $id): JsonResponse
    {
        $driver = Driver::find($id);

        if (! $driver) {
            return response()->json(['success' => false, 'message' => 'Driver tidak ditemukan'], 404);
        }

        $validated = $request->validate([
            'status' => ['required', 'string', 'in:on_duty,off'],
        ]);

        $driver->update(['status' => $validated['status']]);

        return response()->json(['success' => true, 'data' => $driver]);
    }

    public function destroy(string $id): JsonResponse
    {
        $driver = Driver::find($id);

        if (! $driver) {
            return response()->json(['success' => false, 'message' => 'Driver tidak ditemukan'], 404);
        }

        $driver->delete();

        return response()->json(['success' => true, 'message' => 'Driver dihapus']);
    }
}
