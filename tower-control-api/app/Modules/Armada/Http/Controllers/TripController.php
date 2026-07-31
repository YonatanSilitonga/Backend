<?php

namespace App\Modules\Armada\Http\Controllers;

use App\Http\Controllers\Controller;
use App\Modules\Armada\Models\Trip;
use Illuminate\Http\JsonResponse;
use Illuminate\Http\Request;

class TripController extends Controller
{
    public function index(Request $request): JsonResponse
    {
        $trips = Trip::query()
            ->when($request->query('status'), fn ($q, $s) => $q->where('status', $s))
            ->when($request->query('vehicle_id'), fn ($q, $v) => $q->where('vehicle_id', $v))
            ->when($request->query('driver_id'), fn ($q, $d) => $q->where('driver_id', $d))
            ->orderByDesc('created_at')
            ->get();

        return response()->json(['success' => true, 'data' => $trips]);
    }

    public function store(Request $request): JsonResponse
    {
        $validated = $request->validate([
            'kode' => ['required', 'string', 'max:50'],
            'vehicle_id' => ['required', 'string'],
            'driver_id' => ['required', 'string'],
            'asal' => ['required', 'string', 'max:255'],
            'tujuan' => ['required', 'string', 'max:255'],
            'jarak_km' => ['nullable', 'numeric', 'min:0'],
            'status' => ['sometimes', 'string', 'in:planned,in_progress,completed,cancelled'],
        ]);

        $trip = Trip::create($validated);

        return response()->json(['success' => true, 'data' => $trip], 201);
    }

    public function show(string $id): JsonResponse
    {
        $trip = Trip::with(['vehicle', 'driver'])->find($id);

        if (! $trip) {
            return response()->json(['success' => false, 'message' => 'Trip tidak ditemukan'], 404);
        }

        return response()->json(['success' => true, 'data' => $trip]);
    }

    public function update(Request $request, string $id): JsonResponse
    {
        $trip = Trip::find($id);

        if (! $trip) {
            return response()->json(['success' => false, 'message' => 'Trip tidak ditemukan'], 404);
        }

        $validated = $request->validate([
            'kode' => ['sometimes', 'string', 'max:50'],
            'vehicle_id' => ['sometimes', 'string'],
            'driver_id' => ['sometimes', 'string'],
            'asal' => ['sometimes', 'string', 'max:255'],
            'tujuan' => ['sometimes', 'string', 'max:255'],
            'jarak_km' => ['nullable', 'numeric', 'min:0'],
            'status' => ['sometimes', 'string', 'in:planned,in_progress,completed,cancelled'],
        ]);

        $trip->update($validated);

        return response()->json(['success' => true, 'data' => $trip]);
    }

    public function updateStatus(Request $request, string $id): JsonResponse
    {
        $trip = Trip::find($id);

        if (! $trip) {
            return response()->json(['success' => false, 'message' => 'Trip tidak ditemukan'], 404);
        }

        $validated = $request->validate([
            'status' => ['required', 'string', 'in:planned,in_progress,completed,cancelled'],
        ]);

        $data = ['status' => $validated['status']];

        if ($validated['status'] === 'in_progress') {
            $data['started_at'] = now();
        }

        if ($validated['status'] === 'completed') {
            $data['completed_at'] = now();
        }

        $trip->update($data);

        return response()->json(['success' => true, 'data' => $trip]);
    }

    public function destroy(string $id): JsonResponse
    {
        $trip = Trip::find($id);

        if (! $trip) {
            return response()->json(['success' => false, 'message' => 'Trip tidak ditemukan'], 404);
        }

        $trip->delete();

        return response()->json(['success' => true, 'message' => 'Trip dihapus']);
    }
}
