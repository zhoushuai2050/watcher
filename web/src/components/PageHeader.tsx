import type { ReactNode } from "react";

export default function PageHeader({
  kicker,
  title,
  desc,
  children,
}: {
  kicker: ReactNode;
  title: string;
  desc?: string;
  children?: ReactNode;
}) {
  return (
    <header className="topbar">
      <div>
        <p className="eyebrow">{kicker}</p>
        <h1>{title}</h1>
        {desc ? <p className="lede">{desc}</p> : null}
      </div>
      {children ? <div className="topbar-actions">{children}</div> : null}
    </header>
  );
}
