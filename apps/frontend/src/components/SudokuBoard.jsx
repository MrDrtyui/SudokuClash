export function SudokuBoard({ board, initialBoard, onChange, skinTheme }) {
  return (
    <div className={`grid grid-cols-9 overflow-hidden rounded-[1.75rem] border-2 ${skinTheme?.boardFrame || "border-ink bg-ink"}`}>
      {board.flatMap((row, rowIndex) =>
        row.map((value, colIndex) => {
          const fixed = initialBoard[rowIndex][colIndex] !== 0;
          const thickRight = (colIndex + 1) % 3 === 0 && colIndex !== 8;
          const thickBottom = (rowIndex + 1) % 3 === 0 && rowIndex !== 8;

          return (
            <input
              key={`${rowIndex}-${colIndex}`}
              className={`sudoku-cell h-10 w-full border-b border-r text-center text-base font-bold outline-none sm:h-11 ${
                fixed ? skinTheme?.boardFixed || "bg-wheat text-ink" : skinTheme?.boardEditable || "bg-white text-coral"
              } ${thickRight ? "border-r-2 border-r-ink" : "border-r"} ${
                thickBottom ? "border-b-2 border-b-ink" : "border-b"
              }`}
              type="number"
              min="1"
              max="9"
              value={value || ""}
              disabled={fixed}
              onChange={(event) => onChange(rowIndex, colIndex, event.target.value)}
            />
          );
        })
      )}
    </div>
  );
}
