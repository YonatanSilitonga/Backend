<?php

namespace App\Modules\Tower\Http\Controllers;

use App\Http\Controllers\Controller;
use App\Modules\Tower\Models\Invoice;
use App\Modules\Tower\Models\TowerContract;
use Illuminate\Http\JsonResponse;
use Illuminate\Http\Request;
use Illuminate\Support\Str;

class InvoiceController extends Controller
{
    public function index(Request $request): JsonResponse
    {
        $invoices = Invoice::query()
            ->when($request->query('status'), fn ($q, $s) => $q->where('status', $s))
            ->when($request->query('vendor_id'), fn ($q, $v) => $q->where('vendor_id', $v))
            ->get();

        return response()->json(['success' => true, 'data' => $invoices]);
    }

    /**
     * Generate invoice dari kontrak sewa (billing bulanan).
     */
    public function generate(Request $request): JsonResponse
    {
        $validated = $request->validate([
            'contract_id' => ['required', 'string'],
            'periode' => ['required', 'string', 'max:20'], // misal: 2026-07
        ]);

        $contract = TowerContract::with(['tower', 'vendor'])->find($validated['contract_id']);

        if (! $contract) {
            return response()->json(['success' => false, 'message' => 'Kontrak tidak ditemukan'], 404);
        }

        $exists = Invoice::where('contract_id', $contract->id)
            ->where('periode', $validated['periode'])
            ->first();

        if ($exists) {
            return response()->json(['success' => false, 'message' => 'Invoice untuk periode ini sudah ada'], 422);
        }

        $invoice = Invoice::create([
            'no_invoice' => 'INV-' . strtoupper(Str::random(10)),
            'contract_id' => $contract->id,
            'vendor_id' => $contract->vendor_id,
            'tower_id' => $contract->tower_id,
            'periode' => $validated['periode'],
            'jumlah' => $contract->biaya_bulanan,
            'status' => 'unpaid',
            'due_date' => now()->endOfMonth(),
        ]);

        return response()->json(['success' => true, 'data' => $invoice], 201);
    }

    public function show(string $id): JsonResponse
    {
        $invoice = Invoice::find($id);

        if (! $invoice) {
            return response()->json(['success' => false, 'message' => 'Invoice tidak ditemukan'], 404);
        }

        return response()->json(['success' => true, 'data' => $invoice]);
    }

    /**
     * Tandai invoice lunas.
     */
    public function markPaid(string $id): JsonResponse
    {
        $invoice = Invoice::find($id);

        if (! $invoice) {
            return response()->json(['success' => false, 'message' => 'Invoice tidak ditemukan'], 404);
        }

        $invoice->update([
            'status' => 'paid',
            'paid_at' => now(),
        ]);

        return response()->json(['success' => true, 'data' => $invoice]);
    }

    /**
     * Ringkasan billing: total tagihan, outstanding, overdue.
     */
    public function summary(): JsonResponse
    {
        $total = (float) Invoice::sum('jumlah');
        $paid = (float) Invoice::where('status', 'paid')->sum('jumlah');
        $unpaid = (float) Invoice::where('status', 'unpaid')->sum('jumlah');
        $overdue = (float) Invoice::where('status', 'unpaid')
            ->where('due_date', '<', now())
            ->sum('jumlah');

        return response()->json([
            'success' => true,
            'data' => [
                'total_billing' => $total,
                'total_lunas' => $paid,
                'total_belum_bayar' => $unpaid,
                'total_overdue' => $overdue,
                'count_overdue' => Invoice::where('status', 'unpaid')->where('due_date', '<', now())->count(),
            ],
        ]);
    }
}
