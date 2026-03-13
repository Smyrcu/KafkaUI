import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { rowClassName } from "@/lib/utils";

interface Column<T> {
  header: string;
  accessorKey?: keyof T;
  cell?: (row: T) => React.ReactNode;
  className?: string;
}

interface DataTableProps<T> {
  columns: Column<T>[];
  data: T[];
  itemName?: string;
  onRowClick?: (row: T) => void;
}

export function DataTable<T>({ columns, data, itemName = "items", onRowClick }: DataTableProps<T>) {
  return (
    <div className="rounded-lg border bg-card animate-scale-in">
      <div className="px-4 py-3 border-b">
        <p className="text-sm text-muted-foreground">
          {data.length} {data.length === 1 ? itemName.replace(/s$/, "") : itemName}
        </p>
      </div>
      <Table>
        <TableHeader>
          <TableRow className="hover:bg-transparent">
            {columns.map((col, i) => (
              <TableHead key={i} className={col.className}>{col.header}</TableHead>
            ))}
          </TableRow>
        </TableHeader>
        <TableBody>
          {data.map((row, i) => (
            <TableRow
              key={i}
              className={`${rowClassName(i)} ${onRowClick ? "cursor-pointer" : ""}`}
              onClick={onRowClick ? () => onRowClick(row) : undefined}
            >
              {columns.map((col, j) => (
                <TableCell key={j} className={col.className}>
                  {col.cell ? col.cell(row) : String((row as Record<string, unknown>)[col.accessorKey as string] ?? "")}
                </TableCell>
              ))}
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  );
}
