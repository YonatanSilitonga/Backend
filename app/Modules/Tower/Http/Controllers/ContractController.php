<?php

namespace App\Modules\Tower\Http\Controllers;

use App\Http\Controllers\Controller;
use App\Modules\Tower\Models\TowerContract;
use Illuminate\Http\JsonResponse;
use Illuminate\Http\Request;
use Illuminate\Support\Str;

class ContractController extends Controller
{
    public function index(Request $request): JsonResponse
    {
        $contracts = TowerContract::query()
            ->when($request->query('status'), fn ($q, $s) => $q->where('status', $s))
            ->when($request->query('tower_id'), fn ($q, $t) => $q->where('tower_id', $t))
            ->when($request->query('vendor_id'), fn ($q, $v) => $q->where('vendor_id', $v))
            ->get();

        return response()->json(['success' => true, 'data' => $contracts]);
    }

    public function store(Request $request): JsonResponse
    {
        $validated = $request->validate([
            'tower_id' => ['required', 'string'],
            'vendor_id' => ['required', 'string'],
            'tipe_sewa' => ['required', 'string', 'max:100'],
            'biaya_bulanan' => ['required', 'numeric', 'min:0'],
            'tanggal_mulai' => ['required', 'date'],
            'tanggal_selesai' => ['required', 'date', 'after:tanggal_mulai'],
            'status' => ['sometimes', 'string', 'in:active,expired,terminated'],
        ]);

        $validated['kode'] = 'KTR-' . strtoupper(Str::random(8));
        $validated['status'] = $validated['status'] ?? 'active';

        $contract = TowerContract::create($validated);

        return response()->json(['success' => true, 'data' => $contract], 201);
    }

    public function show(string $id): JsonResponse
    {
        $contract = TowerContract::with(['tower', 'vendor'])->find($id);

        if (! $contract) {
            return response()->json(['success' => false, 'message' => 'Kontrak tidak ditemukan'], 404);
        }

        return response()->json(['success' => true, 'data' => $contract]);
    }

    public function update(Request $request, string $id): JsonResponse
    {
        $contract = TowerContract::find($id);

        if (! $contract) {
            return response()->json(['success' => false, 'message' => 'Kontrak tidak ditemukan'], 404);
        }

        $validated = $request->validate([
            'tower_id' => ['sometimes', 'string'],
            'vendor_id' => ['sometimes', 'string'],
            'tipe_sewa' => ['sometimes', 'string', 'max:100'],
            'biaya_bulanan' => ['sometimes', 'numeric', 'min:0'],
            'tanggal_mulai' => ['sometimes', 'date'],
            'tanggal_selesai' => ['sometimes', 'date'],
            'status' => ['sometimes', 'string', 'in:active,expired,terminated'],
        ]);

        $contract->update($validated);

        return response()->json(['success' => true, 'data' => $contract]);
    }

    public function destroy(string $id): JsonResponse
    {
        $contract = TowerContract::find($id);

        if (! $contract) {
            return response()->json(['success' => false, 'message' => 'Kontrak tidak ditemukan'], 404);
        }

        $contract->delete();

        return response()->json(['success' => true, 'message' => 'Kontrak dihapus']);
    }
}
