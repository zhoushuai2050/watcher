import { useEffect, useState } from "react";

export function useQueryPage() {
  const [input, setInput] = useState("");
  const [q, setQ] = useState("");
  const [page, setPage] = useState(1);

  useEffect(() => {
    const id = setTimeout(() => setQ(input.trim()), 300);
    return () => clearTimeout(id);
  }, [input]);

  useEffect(() => {
    setPage(1);
  }, [q]);

  return { input, setInput, q, page, setPage };
}

export default function SearchPager({
  input,
  onInput,
  placeholder,
  page,
  pageSize,
  total,
  onPage,
  hideNav,
}: {
  input: string;
  onInput: (v: string) => void;
  placeholder?: string;
  page: number;
  pageSize: number;
  total: number;
  onPage: (p: number) => void;
  hideNav?: boolean;
}) {
  const pages = Math.max(1, Math.ceil((total || 0) / (pageSize || 1)));
  useEffect(() => {
    if (!hideNav && page > pages) onPage(pages);
  }, [page, pages, onPage, hideNav]);

  return (
    <div className="pager">
      <input value={input} onChange={(e) => onInput(e.target.value)} placeholder={placeholder || "模糊搜索"} />
      {hideNav ? (
        <span className="muted">{total > pageSize ? `最近 ${pageSize} 条 / 共 ${total} 条` : `${total} 条`}</span>
      ) : (
        <div className="pager-nav">
          <button className="btn btn-ghost" disabled={page <= 1} onClick={() => onPage(page - 1)}>
            上一页
          </button>
          <span className="muted">
            {page} / {pages}
          </span>
          <button className="btn btn-ghost" disabled={page >= pages} onClick={() => onPage(page + 1)}>
            下一页
          </button>
          <span className="muted">{total} 条</span>
        </div>
      )}
    </div>
  );
}
