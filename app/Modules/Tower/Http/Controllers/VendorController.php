<?php

namespace App\Modules\Tower\Http\Controllers;

use App\Http\Controllers\Controller;
use App\Modules\Tower\Models\Vendor;
use Illuminate\Http\JsonResponse;
use Illuminate\Http\Request;

class VendorController extends Controller
{
    public function index(Request $request): JsonResponse
    {
        $vendors = Vendor::query()
            ->when($request->query('status'), fn ($q, $s) => $q->where('status', $s))
            ->get();

        return response()->json(['success' => true, 'data' => $vendors]);
    }

    public function store(Request $request): JsonResponse
    {
        $validated = $request->validate([
            'nama' => ['required', 'string', 'max:255'],
            'kontak' => ['nullable', 'string', 'max:255'],
            'telepon' => ['nullable', 'string', 'max:20'],
            'spesialisasi' => ['nullable', 'string', 'max:255'],
            'status' => ['sometimes', 'string', 'in:active,inactive'],
        ]);

        $vendor = Vendor::create($validated);

        return response()->json(['success' => true, 'data' => $vendor], 201);
    }

    public function show(string $id): JsonResponse
    {
        $vendor = Vendor::find($id);

        if (! $vendor) {
            return response()->json(['success' => false, 'message' => 'Vendor tidak ditemukan'], 404);
        }

        return response()->json(['success' => true, 'data' => $vendor]);
    }

    public function update(Request $request, string $id): JsonResponse
    {
        $vendor = Vendor::find($id);

        if (! $vendor) {
            return response()->json(['success' => false, 'message' => 'Vendor tidak ditemukan'], 404);
        }

        $validated = $request->validate([
            'nama' => ['sometimes', 'string', 'max:255'],
            'kontak' => ['nullable', 'string', 'max:255'],
            'telepon' => ['nullable', 'string', 'max:20'],
            'spesialisasi' => ['nullable', 'string', 'max:255'],
            'status' => ['sometimes', 'string', 'in:active,inactive'],
        ]);

        $vendor->update($validated);

        return response()->json(['success' => true, 'data' => $vendor]);
    }

    public function destroy(string $id): JsonResponse
    {
        $vendor = Vendor::find($id);

        if (! $vendor) {
            return response()->json(['success' => false, 'message' => 'Vendor tidak ditemukan'], 404);
        }

        $vendor->delete();

        return response()->json(['success' => true, 'message' => 'Vendor dihapus']);
    }
}
